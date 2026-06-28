package policy

import (
	"container/list"
	"context"
	"crypto/sha1" //nolint:gosec // SHA-1 は HIBP Range API のプロトコル要件であり、暗号強度には依存しない。
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HIBP Range API への接続パラメータ (ADR-028 §3)。
const (
	hibpDefaultEndpoint    = "https://api.pwnedpasswords.com/range/"
	hibpDefaultTimeout     = 2 * time.Second
	hibpCacheTTL           = 10 * time.Minute
	hibpCacheCapacity      = 1024
	hibpMaxResponseBytes   = 4 << 20 // padding 込みでも数百 KiB。上限を 4 MiB に固定して読み過ぎを防ぐ。
	hibpFailuresMetricName = "breached_password_checker_failures_total"
)

// HibpBreachedPasswordChecker は HIBP Range API (k-anonymity) で漏洩 password を
// 検査する ports.BreachedPasswordChecker 実装 (ADR-028 §3)。
//
// 生 password は外部送信せず、SHA-1 の先頭 5 文字 (prefix) のみを送る。HTTP error /
// timeout / 5xx は false を返す fail-open とし (ADR-028 §4)、失敗は warn log と
// metric 1 本で可視化する。
type HibpBreachedPasswordChecker struct {
	endpoint  string
	userAgent string
	client    *http.Client
	cache     *prefixCache
	failures  metric.Int64Counter
}

// HibpOption は HibpBreachedPasswordChecker の構成を差し替える。テストで endpoint /
// clock / meter を注入するために用いる。
type HibpOption func(*hibpConfig)

type hibpConfig struct {
	endpoint string
	client   *http.Client
	meter    metric.Meter
	now      func() time.Time
}

// WithHibpEndpoint は Range API のベース URL を差し替える (テスト用)。
func WithHibpEndpoint(endpoint string) HibpOption {
	return func(c *hibpConfig) { c.endpoint = endpoint }
}

// WithHibpHTTPClient は HTTP クライアントを差し替える (テスト用)。
func WithHibpHTTPClient(client *http.Client) HibpOption {
	return func(c *hibpConfig) { c.client = client }
}

// WithHibpMeter は failure metric の出力先 meter を差し替える (テスト用)。
func WithHibpMeter(meter metric.Meter) HibpOption {
	return func(c *hibpConfig) { c.meter = meter }
}

// NewHibpBreachedPasswordChecker は本番構成の HIBP checker を組み立てる。
// version は User-Agent (HIBP の etiquette) に乗せる。
func NewHibpBreachedPasswordChecker(version string, opts ...HibpOption) *HibpBreachedPasswordChecker {
	cfg := hibpConfig{
		endpoint: hibpDefaultEndpoint,
		client:   &http.Client{Timeout: hibpDefaultTimeout},
		meter:    otel.Meter("ra-idp-go"),
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	failures, _ := cfg.meter.Int64Counter(
		hibpFailuresMetricName,
		metric.WithDescription("HIBP breached password lookup failures (fail-open)."),
	)
	return &HibpBreachedPasswordChecker{
		endpoint:  cfg.endpoint,
		userAgent: "ra-idp-go/" + version,
		client:    cfg.client,
		cache:     newPrefixCache(hibpCacheTTL, hibpCacheCapacity, cfg.now),
		failures:  failures,
	}
}

// IsBreached は password が HIBP に登録された漏洩 password か判定する。失敗時は
// false を返す (fail-open)。
func (c *HibpBreachedPasswordChecker) IsBreached(ctx context.Context, password string) bool {
	sum := sha1.Sum([]byte(password)) //nolint:gosec // プロトコル要件 (ADR-028 §3)。
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := digest[:5], digest[5:]

	suffixes, ok := c.cache.get(prefix)
	if !ok {
		fetched, err := c.fetch(ctx, prefix)
		if err != nil {
			c.recordFailure(ctx, err)
			return false
		}
		c.cache.put(prefix, fetched)
		suffixes = fetched
	}
	_, breached := suffixes[suffix]
	return breached
}

// hibpStatusError は 200 以外のレスポンスを表す。failure reason 分類に使う。
type hibpStatusError struct{ status int }

func (e *hibpStatusError) Error() string {
	return fmt.Sprintf("hibp: unexpected status %d", e.status)
}

func (c *HibpBreachedPasswordChecker) fetch(ctx context.Context, prefix string) (map[string]struct{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+prefix, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	// Add-Padding はレスポンスサイズ side-channel を抑える HIBP 推奨ヘッダ (ADR-028 §3)。
	req.Header.Set("Add-Padding", "true")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &hibpStatusError{status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, hibpMaxResponseBytes))
	if err != nil {
		return nil, err
	}
	return parseHibpRange(string(body)), nil
}

// parseHibpRange は "SUFFIX:COUNT" 行の繰り返しから count>0 の suffix 集合を作る。
// Add-Padding が混ぜる count=0 のダミー行は除外する。
func parseHibpRange(body string) map[string]struct{} {
	out := make(map[string]struct{})
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		suffix, count, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil || n <= 0 {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(suffix))] = struct{}{}
	}
	return out
}

func (c *HibpBreachedPasswordChecker) recordFailure(ctx context.Context, err error) {
	reason := classifyHibpFailure(err)
	// privacy: plaintext / SHA-1 suffix は出さない。reason と err (status / 接続情報のみ) に留める。
	log.Printf("breached password checker: hibp lookup failed (reason=%s): %v", reason, err)
	if c.failures != nil {
		c.failures.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
	}
}

func classifyHibpFailure(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if statusErr, ok := errors.AsType[*hibpStatusError](err); ok {
		if statusErr.status >= 500 {
			return "server_error"
		}
		return "client_error"
	}
	return "connection_error"
}

// prefixCache は prefix→suffix集合 を保持する TTL 付き LRU。HIBP への往復を抑える。
type prefixCache struct {
	mu       sync.Mutex
	ttl      time.Duration
	capacity int
	now      func() time.Time
	entries  map[string]*list.Element
	order    *list.List // front=most-recently-used
}

type cacheItem struct {
	prefix    string
	suffixes  map[string]struct{}
	expiresAt time.Time
}

func newPrefixCache(ttl time.Duration, capacity int, now func() time.Time) *prefixCache {
	return &prefixCache{
		ttl:      ttl,
		capacity: capacity,
		now:      now,
		entries:  make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (c *prefixCache) get(prefix string) (map[string]struct{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.entries[prefix]
	if !ok {
		return nil, false
	}
	item, _ := elem.Value.(*cacheItem)
	if c.now().After(item.expiresAt) {
		c.order.Remove(elem)
		delete(c.entries, prefix)
		return nil, false
	}
	c.order.MoveToFront(elem)
	return item.suffixes, true
}

func (c *prefixCache) put(prefix string, suffixes map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiresAt := c.now().Add(c.ttl)
	if elem, ok := c.entries[prefix]; ok {
		if item, _ := elem.Value.(*cacheItem); item != nil {
			item.suffixes = suffixes
			item.expiresAt = expiresAt
		}
		c.order.MoveToFront(elem)
		return
	}
	elem := c.order.PushFront(&cacheItem{prefix: prefix, suffixes: suffixes, expiresAt: expiresAt})
	c.entries[prefix] = elem
	if c.order.Len() > c.capacity {
		if oldest := c.order.Back(); oldest != nil {
			c.order.Remove(oldest)
			if item, _ := oldest.Value.(*cacheItem); item != nil {
				delete(c.entries, item.prefix)
			}
		}
	}
}
