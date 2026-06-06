// 認可リクエストのドメインモデル。prompt / max_age / id_token_hint 制御を含む。
package domain

import (
	"strings"
	"time"

	"ra-idp-go/internal/spec"
)

// AuthorizationRequestPolicy は prompt / max_age / id_token_hint 等の OIDC 規定値による
// 再認証必要性判断をまとめる。
type AuthorizationRequestPolicy struct {
	Prompt      *string
	MaxAge      *int
	IDTokenHint *string
}

// NeedsReauthentication は context (現セッション) が要件を満たすか判定する。
//   - prompt=login: 常に true
//   - prompt=none: 認証されていない場合は呼び出し側で access_denied
//   - max_age: auth_time が古ければ true
//   - id_token_hint: 別ユーザー対象なら true (本実装ではプロト簡略化のため未検査)
func NeedsReauthentication(p AuthorizationRequestPolicy, authTime, now time.Time, promptLoginSatisfied bool) bool {
	if p.Prompt != nil {
		switch *p.Prompt {
		case "login":
			return !promptLoginSatisfied
		case "none":
			return false
		}
	}
	if p.MaxAge != nil {
		maxAge := time.Duration(*p.MaxAge) * time.Second
		if now.Sub(authTime) >= maxAge {
			return true
		}
	}
	return false
}

func ParsePrompt(req *spec.AuthorizationRequest) AuthorizationRequestPolicy {
	return AuthorizationRequestPolicy{
		Prompt:      req.Prompt,
		MaxAge:      req.MaxAge,
		IDTokenHint: nil,
	}
}

// ScopeIntersection は scope 文字列を要素分割して共通部分を返す。
func ScopeIntersection(requested, allowed string) []string {
	allow := map[string]bool{}
	for _, s := range strings.Fields(allowed) {
		allow[s] = true
	}
	var out []string
	for _, s := range strings.Fields(requested) {
		if allow[s] {
			out = append(out, s)
		}
	}
	return out
}
