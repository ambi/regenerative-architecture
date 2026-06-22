// Token Exchange Grant (RFC 8693) のユースケース。
//
// 本実装は ADR-049 に忠実に、以下に限定する (fail-closed):
//   - SELF-ISSUED トークンのみ: subject_token / actor_token は本 IdP が発行し、
//     既存の IntrospectAccessToken (署名検証 + active) を通過したものに限る。
//     外部/フェデレーショントークンは対象外 (将来 wi-54 / wi-57)。
//   - DELEGATION ONLY: 発行トークンの sub は subject_token.sub を維持し、必ず act を
//     設定する。impersonation (act 省略 / sub 差し替え) は対象外 (将来、gated)。
//   - MAX DELEGATION DEPTH: act の入れ子は MaxDelegationDepth まで。超過は invalid_request。
//     (テナント別上書きは将来)
//   - may_act 強制: subject_token に may_act があれば現在アクター sub が may_act.sub と
//     一致しなければ拒否。
//   - RESOURCE INDICATORS (constrained RFC 8707): resource を必須・1 個のみとし、
//     発行トークン aud = [resource] とする。resource-server 登録モデルは未導入 (TOFU)。
//   - REFRESH TOKEN は発行しない。
package usecases

import (
	"context"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

// MaxDelegationDepth は発行トークンの act 入れ子の最大深さ (ADR-049)。
const MaxDelegationDepth = 3

const (
	tokenTypeAccessTokenURN = "urn:ietf:params:oauth:token-type:access_token"
)

type ExchangeTokenInput struct {
	ClientID           string
	SubjectToken       string
	SubjectTokenType   string
	ActorToken         string
	ActorTokenType     string
	Resource           []string // form の resource (複数指定され得るため slice で受ける)
	Scope              string
	RequestedTokenType string
	ProofJKT           string
	ProofX5TS256       string
}

type ExchangeTokenResult struct {
	AccessToken     string
	IssuedTokenType string
	TokenType       string
	ExpiresIn       int
	Scope           string
}

type ExchangeTokenDeps struct {
	ClientRepo   ports.ClientRepository
	Introspector ports.TokenIntrospector
	TokenIssuer  ports.TokenIssuer
	Emit         func(spec.DomainEvent)
}

func ExchangeToken(ctx context.Context, deps ExchangeTokenDeps, in ExchangeTokenInput, now time.Time) (*ExchangeTokenResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tenantID := tenancy.TenantID(ctx)

	reject := func(actorSub string, err *OAuthError) (*ExchangeTokenResult, error) {
		emit(deps.Emit, &spec.TokenExchangeRejected{At: now, TenantID: tenantID, ActorSub: actorSub, Reason: err.Code})
		return nil, err
	}

	// --- resource (RFC 8707, constrained): 必須・1 個のみ ---
	resources := nonEmpty(in.Resource)
	if len(resources) == 0 {
		return reject("", NewOAuthError("invalid_request", "resource パラメータが必要です"))
	}
	if len(resources) > 1 {
		return reject("", NewOAuthError("invalid_request", "resource は 1 個のみ指定できます (1 token = 1 resource)"))
	}
	resource := resources[0]

	// --- subject_token (必須) ---
	if in.SubjectToken == "" {
		return reject("", NewOAuthError("invalid_request", "subject_token が必要です"))
	}
	if in.SubjectTokenType != "" && in.SubjectTokenType != tokenTypeAccessTokenURN {
		// SELF-ISSUED access_token のみ対応 (外部トークンは対象外)。
		return reject("", NewOAuthError("invalid_request", "未対応の subject_token_type です"))
	}
	subject, err := deps.Introspector.IntrospectAccessToken(ctx, in.SubjectToken)
	if err != nil {
		return nil, err
	}
	if subject == nil || !subject.Active {
		return reject("", NewOAuthError("invalid_grant", "subject_token は無効または失効しています"))
	}

	// --- actor_token (任意) ---
	currentActorSub := in.ClientID
	if in.ActorToken != "" {
		if in.ActorTokenType != "" && in.ActorTokenType != tokenTypeAccessTokenURN {
			return reject("", NewOAuthError("invalid_request", "未対応の actor_token_type です"))
		}
		actor, err := deps.Introspector.IntrospectAccessToken(ctx, in.ActorToken)
		if err != nil {
			return nil, err
		}
		if actor == nil || !actor.Active {
			return reject("", NewOAuthError("invalid_grant", "actor_token は無効または失効しています"))
		}
		currentActorSub = actor.Sub
	}
	if currentActorSub == "" {
		return reject("", NewOAuthError("invalid_request", "現在のアクターを決定できません"))
	}

	// --- may_act 強制 (fail-closed) ---
	if subject.MayAct != nil {
		mayActSub, _ := subject.MayAct["sub"].(string)
		if mayActSub == "" || mayActSub != currentActorSub {
			return reject(currentActorSub, NewOAuthError("invalid_grant", "現在のアクターは may_act で許可されていません"))
		}
	}

	// --- act チェーン構築 (RFC 8693 §4.1) ---
	// act = {"sub": currentActor}; subject_token に act があれば入れ子で連結する。
	act := map[string]any{"sub": currentActorSub}
	if subject.Act != nil {
		act["act"] = subject.Act
	}
	depth := actDepth(act)
	if depth > MaxDelegationDepth {
		return reject(currentActorSub, NewOAuthError("invalid_request", "委任の深さが上限を超えています"))
	}

	// --- scope ダウンスコープ (拡大不可) ---
	subjectScopes := strings.Fields(subject.Scope)
	grantedScopes := subjectScopes
	if requested := strings.Fields(in.Scope); len(requested) > 0 {
		subset := map[string]bool{}
		for _, s := range subjectScopes {
			subset[s] = true
		}
		for _, s := range requested {
			if !subset[s] {
				return reject(currentActorSub, NewOAuthError("invalid_scope", "subject_token のスコープを超える要求です"))
			}
		}
		grantedScopes = requested
	}

	// --- client の解決 (AuthZEN gate は token_handler 側の grant 宣言チェックで担保) ---
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return reject(currentActorSub, NewOAuthError("invalid_client", "未知の client_id"))
	}

	// --- sender constraint (DPoP / mTLS) ---
	var sc *spec.SenderConstraint
	if in.ProofJKT != "" {
		sc = &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: in.ProofJKT}
	} else if in.ProofX5TS256 != "" {
		sc = &spec.SenderConstraint{Type: spec.SenderConstraintMTLS, X5TS256: in.ProofX5TS256}
	}

	// --- 発行 (DELEGATION ONLY: sub = subject.sub, aud = [resource], act 必須) ---
	access, jti, err := deps.TokenIssuer.SignAccessToken(ctx, ports.AccessTokenInput{
		Client: client, Sub: subject.Sub, Scopes: grantedScopes,
		SenderConstraint: sc, AuthTime: now.Unix(),
		Audiences: []string{resource}, Act: act,
	})
	if err != nil {
		return nil, err
	}

	emit(deps.Emit, &spec.AccessTokenIssued{
		At: now, TenantID: tenantID, JTI: jti, ClientID: client.ClientID,
		Sub: subject.Sub, Scopes: grantedScopes, SenderConstraint: senderConstraintTag(sc),
	})
	emit(deps.Emit, &spec.TokenExchanged{
		At: now, TenantID: tenantID, ActorSub: currentActorSub, SubjectSub: subject.Sub,
		Audience: resource, DelegationDepth: depth,
	})

	tokenType := "Bearer"
	if sc != nil && sc.Type == spec.SenderConstraintDPoP {
		tokenType = "DPoP"
	}
	return &ExchangeTokenResult{
		AccessToken:     access,
		IssuedTokenType: tokenTypeAccessTokenURN,
		TokenType:       tokenType,
		ExpiresIn:       deps.TokenIssuer.AccessTokenTTLSeconds(),
		Scope:           strings.Join(grantedScopes, " "),
	}, nil
}

// actDepth は act claim の入れ子の深さを数える。最も外側の act を 1 とする。
func actDepth(act map[string]any) int {
	depth := 0
	for act != nil {
		depth++
		nested, ok := act["act"].(map[string]any)
		if !ok {
			break
		}
		act = nested
	}
	return depth
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
