package policy

import (
	"context"
	"crypto/sha1" //nolint:gosec // テストでも HIBP プロトコルの SHA-1 を再現する。
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func sha1PrefixSuffix(password string) (prefix, suffix string) {
	sum := sha1.Sum([]byte(password)) //nolint:gosec // プロトコル要件。
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))
	return digest[:5], digest[5:]
}

func TestHibpIsBreachedReportsMatch(t *testing.T) {
	const password = "password"
	prefix, suffix := sha1PrefixSuffix(password)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/"+prefix {
			t.Errorf("unexpected path %q, want /%s", got, prefix)
		}
		if got := r.Header.Get("Add-Padding"); got != "true" {
			t.Errorf("Add-Padding=%q, want true", got)
		}
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "ra-idp-go/") {
			t.Errorf("unexpected User-Agent %q", ua)
		}
		// 別 suffix の本物ヒット + 対象 suffix を count>0 で返す。
		fmt.Fprintf(w, "00000000000000000000000000000000001:1\r\n%s:42\r\n", suffix)
	}))
	defer srv.Close()

	checker := NewHibpBreachedPasswordChecker("test",
		WithHibpEndpoint(srv.URL+"/"), WithHibpHTTPClient(srv.Client()))
	if !checker.IsBreached(context.Background(), password) {
		t.Fatal("expected breached=true for matched suffix")
	}
}

func TestHibpIsBreachedIgnoresPaddingAndMisses(t *testing.T) {
	const password = "a-very-unique-passphrase-2026"
	_, suffix := sha1PrefixSuffix(password)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 対象 suffix は count=0 (Add-Padding のダミー)、別 suffix のみ count>0。
		fmt.Fprintf(w, "%s:0\r\n00000000000000000000000000000000002:7\r\n", suffix)
	}))
	defer srv.Close()

	checker := NewHibpBreachedPasswordChecker("test",
		WithHibpEndpoint(srv.URL+"/"), WithHibpHTTPClient(srv.Client()))
	if checker.IsBreached(context.Background(), password) {
		t.Fatal("expected breached=false: suffix only present with count=0")
	}
}

func TestHibpFailOpenOnServerError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	checker := NewHibpBreachedPasswordChecker("test",
		WithHibpEndpoint(srv.URL+"/"), WithHibpHTTPClient(srv.Client()),
		WithHibpMeter(provider.Meter("test")))
	if checker.IsBreached(context.Background(), "password") {
		t.Fatal("expected fail-open (false) on 5xx")
	}
	if got := failureCount(t, reader, "server_error"); got != 1 {
		t.Fatalf("server_error failure metric=%d, want 1", got)
	}
}

func TestHibpFailOpenOnTimeout(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, "00000000000000000000000000000000003:1\r\n")
	}))
	defer srv.Close()

	checker := NewHibpBreachedPasswordChecker("test",
		WithHibpEndpoint(srv.URL+"/"),
		WithHibpHTTPClient(&http.Client{Timeout: 30 * time.Millisecond}),
		WithHibpMeter(provider.Meter("test")))
	if checker.IsBreached(context.Background(), "password") {
		t.Fatal("expected fail-open (false) on timeout")
	}
	if got := failureCount(t, reader, "timeout"); got != 1 {
		t.Fatalf("timeout failure metric=%d, want 1", got)
	}
}

func TestHibpFailOpenOnConnectionError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// port 1 は接続拒否される。
	checker := NewHibpBreachedPasswordChecker("test",
		WithHibpEndpoint("http://127.0.0.1:1/"),
		WithHibpHTTPClient(&http.Client{Timeout: 500 * time.Millisecond}),
		WithHibpMeter(provider.Meter("test")))
	if checker.IsBreached(context.Background(), "password") {
		t.Fatal("expected fail-open (false) on connection error")
	}
	if got := failureCount(t, reader, "connection_error"); got != 1 {
		t.Fatalf("connection_error failure metric=%d, want 1", got)
	}
}

func TestHibpCachesPrefixLookups(t *testing.T) {
	const password = "password"
	_, suffix := sha1PrefixSuffix(password)
	var requests atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprintf(w, "%s:42\r\n", suffix)
	}))
	defer srv.Close()

	checker := NewHibpBreachedPasswordChecker("test",
		WithHibpEndpoint(srv.URL+"/"), WithHibpHTTPClient(srv.Client()))
	for range 3 {
		if !checker.IsBreached(context.Background(), password) {
			t.Fatal("expected breached=true")
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests=%d, want 1 (subsequent lookups must hit cache)", got)
	}
}

func TestPrefixCacheEvictsByTTL(t *testing.T) {
	now := time.Unix(0, 0)
	cache := newPrefixCache(time.Minute, 8, func() time.Time { return now })
	cache.put("ABCDE", map[string]struct{}{"X": {}})

	if _, ok := cache.get("ABCDE"); !ok {
		t.Fatal("entry should be present before TTL")
	}
	now = now.Add(2 * time.Minute)
	if _, ok := cache.get("ABCDE"); ok {
		t.Fatal("entry should be evicted after TTL")
	}
}

func failureCount(t *testing.T, reader sdkmetric.Reader, reason string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != hibpFailuresMetricName {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				if v, ok := dp.Attributes.Value(attribute.Key("reason")); ok && v.AsString() == reason {
					return dp.Value
				}
			}
		}
	}
	return 0
}
