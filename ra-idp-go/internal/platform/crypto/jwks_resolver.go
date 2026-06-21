package crypto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"ra-idp-go/internal/spec"
)

const maxJWKSBytes = 1 << 20

type cachedJWKS struct {
	keys      []map[string]any
	expiresAt time.Time
}

type JWKResolver struct {
	mu       sync.Mutex
	cache    map[string]cachedJWKS
	resolver *net.Resolver
	client   *http.Client
}

func NewJWKResolver() *JWKResolver {
	r := &JWKResolver{cache: map[string]cachedJWKS{}, resolver: net.DefaultResolver}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := r.safeIPs(ctx, host)
			if err != nil {
				return nil, err
			}
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(
				ctx,
				network,
				net.JoinHostPort(ips[0].String(), port),
			)
		},
		TLSHandshakeTimeout: 2 * time.Second,
	}
	r.client = &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("jwks_uri: too many redirects")
			}
			return ValidateJWKSURI(req.URL.String())
		},
	}
	return r
}

func ValidateJWKSURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("jwks_uri: %w", err)
	}
	if parsed.Scheme != "https" {
		return errors.New("jwks_uri: https is required")
	}
	if parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("jwks_uri: invalid authority, userinfo, or fragment")
	}
	return nil
}

func (r *JWKResolver) Resolve(ctx context.Context, client *spec.Client) ([]map[string]any, error) {
	if keys, err := InlineJWKs(client.JWKS); err == nil {
		return keys, nil
	}
	if client.JwksURI == nil || *client.JwksURI == "" {
		return nil, errors.New("private_key_jwt client has no jwks or jwks_uri")
	}
	return r.fetch(ctx, *client.JwksURI)
}

func InlineJWKs(jwks map[string]any) ([]map[string]any, error) {
	var raw []any
	switch keys := jwks["keys"].(type) {
	case []any:
		raw = keys
	case []map[string]any:
		raw = make([]any, len(keys))
		for i := range keys {
			raw[i] = keys[i]
		}
	default:
		return nil, errors.New("inline jwks is missing keys")
	}
	if len(raw) == 0 {
		return nil, errors.New("inline jwks is empty")
	}
	keys := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		key, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("inline jwks contains a non-object key")
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *JWKResolver) fetch(ctx context.Context, raw string) ([]map[string]any, error) {
	if err := ValidateJWKSURI(raw); err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(raw)
	if _, err := r.safeIPs(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}
	r.mu.Lock()
	cached, ok := r.cache[raw]
	r.mu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.keys, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks_uri: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks_uri: status %d", resp.StatusCode)
	}
	var document map[string]any
	reader := io.LimitReader(resp.Body, maxJWKSBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) > maxJWKSBytes {
		return nil, errors.New("jwks_uri response exceeds 1 MiB")
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode jwks_uri: %w", err)
	}
	keys, err := InlineJWKs(document)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[raw] = cachedJWKS{keys: keys, expiresAt: time.Now().Add(5 * time.Minute)}
	r.mu.Unlock()
	return keys, nil
}

func (r *JWKResolver) safeIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return nil, errors.New("jwks_uri resolves to a non-public address")
		}
		return []net.IP{ip}, nil
	}
	addresses, err := r.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve jwks_uri host: %w", err)
	}
	var out []net.IP
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return nil, errors.New("jwks_uri resolves to a non-public address")
		}
		out = append(out, address.IP)
	}
	if len(out) == 0 {
		return nil, errors.New("jwks_uri host has no addresses")
	}
	return out, nil
}

func isPublicIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast() &&
		!strings.HasPrefix(ip.String(), "100.64.")
}
