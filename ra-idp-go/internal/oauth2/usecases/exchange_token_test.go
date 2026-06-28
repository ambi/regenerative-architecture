package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"
)

type denyAuthorizer struct {
	last spec.AuthZRequest
}

func (d *denyAuthorizer) Authorize(_ context.Context, req spec.AuthZRequest) (spec.AuthZResponse, error) {
	d.last = req
	return spec.AuthZResponse{Permit: false, Reasons: []string{"test_policy_denied"}}, nil
}

// recordingIssuer は SignAccessToken の入力を記録するスタブ。
type recordingIssuer struct {
	lastInput ports.AccessTokenInput
	calls     int
}

func (r *recordingIssuer) SignAccessToken(_ context.Context, in ports.AccessTokenInput) (string, string, error) {
	r.calls++
	r.lastInput = in
	return "exchanged-access-token", "jti-exch", nil
}

func (r *recordingIssuer) SignIDToken(context.Context, ports.IDTokenInput) (string, error) {
	return "", nil
}
func (r *recordingIssuer) AccessTokenTTLSeconds() int { return 600 }
func (r *recordingIssuer) IDTokenTTLSeconds() int     { return 3600 }

// tokenIntrospector は token 文字列ごとに異なる結果を返すスタブ。
type tokenIntrospector struct {
	results map[string]*ports.IntrospectionResult
}

func (s tokenIntrospector) IntrospectAccessToken(_ context.Context, token string) (*ports.IntrospectionResult, error) {
	if r, ok := s.results[token]; ok {
		return r, nil
	}
	return &ports.IntrospectionResult{Active: false}, nil
}

func newExchangeTokenDeps(t *testing.T, issuer *recordingIssuer, results map[string]*ports.IntrospectionResult) ExchangeTokenDeps {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	clientRepo.Seed(&spec.OAuth2Client{
		ClientID:   "client",
		ClientType: spec.ClientConfidential,
		GrantTypes: []spec.GrantType{spec.GrantTokenExchange},
		Scope:      "read write",
		CreatedAt:  time.Now().UTC(),
	})
	return ExchangeTokenDeps{
		ClientRepo:   clientRepo,
		Introspector: tokenIntrospector{results: results},
		TokenIssuer:  issuer,
	}
}

func TestExchangeTokenBuildsActAndAudience(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read write"},
	})
	res, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	if res.IssuedTokenType != tokenTypeAccessTokenURN {
		t.Fatalf("issued_token_type=%q", res.IssuedTokenType)
	}
	if got := issuer.lastInput.Audiences; len(got) != 1 || got[0] != "https://api.example" {
		t.Fatalf("aud=%v, want [https://api.example]", got)
	}
	if issuer.lastInput.Sub != "user-1" {
		t.Fatalf("sub=%q, want user-1 (delegation keeps subject sub)", issuer.lastInput.Sub)
	}
	act := issuer.lastInput.Act
	if act == nil || act["sub"] != "client" {
		t.Fatalf("act=%v, want act.sub=client (authenticated client is current actor)", act)
	}
	if _, nested := act["act"]; nested {
		t.Fatalf("act は入れ子であってはなりません (subject に act 無し): %v", act)
	}
}

func TestExchangeTokenNestsExistingActChain(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read", Act: map[string]any{"sub": "svc-a"}},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	act := issuer.lastInput.Act
	if act["sub"] != "client" {
		t.Fatalf("outer act.sub=%v, want client", act["sub"])
	}
	nested, ok := act["act"].(map[string]any)
	if !ok || nested["sub"] != "svc-a" {
		t.Fatalf("nested act=%v, want {sub: svc-a}", act["act"])
	}
}

func TestExchangeTokenRejectsExceedingMaxDepth(t *testing.T) {
	// 既存 act が深さ 3 (MaxDelegationDepth) → 交換すると深さ 4 で拒否。
	deep := map[string]any{"sub": "a3", "act": map[string]any{"sub": "a2", "act": map[string]any{"sub": "a1"}}}
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read", Act: deep},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("max depth 超過が拒否されませんでした")
	}
	if issuer.calls != 0 {
		t.Fatal("拒否されたのにトークンが発行されました")
	}
}

func TestExchangeTokenRejectsInactiveSubject(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: false},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("inactive subject_token が拒否されませんでした")
	}
}

func TestExchangeTokenEnforcesMayAct(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		// may_act.sub != currentActor("client") → 拒否。
		"subj": {Active: true, Sub: "user-1", Scope: "read", MayAct: map[string]any{"sub": "other"}},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("may_act 違反が拒否されませんでした")
	}

	// may_act.sub == currentActor → 許可。
	issuer2 := &recordingIssuer{}
	deps2 := newExchangeTokenDeps(t, issuer2, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read", MayAct: map[string]any{"sub": "client"}},
	})
	if _, err := ExchangeToken(context.Background(), deps2, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC()); err != nil {
		t.Fatalf("may_act 一致時に拒否されました: %v", err)
	}
}

func TestExchangeTokenCannotWidenScope(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read"},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Scope: "read write",
		Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("スコープ拡大が拒否されませんでした")
	}
}

func TestExchangeTokenDownscopes(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read write"},
	})
	res, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Scope: "read",
		Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	if res.Scope != "read" {
		t.Fatalf("scope=%q, want read", res.Scope)
	}
}

func TestExchangeTokenRequiresSingleResource(t *testing.T) {
	cases := map[string][]string{
		"missing":  nil,
		"multiple": {"https://a.example", "https://b.example"},
	}
	for name, resource := range cases {
		t.Run(name, func(t *testing.T) {
			issuer := &recordingIssuer{}
			deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
				"subj": {Active: true, Sub: "user-1", Scope: "read"},
			})
			_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
				ClientID: "client", SubjectToken: "subj", Resource: resource,
			}, time.Now().UTC())
			if err == nil {
				t.Fatalf("resource=%v が拒否されませんでした", resource)
			}
		})
	}
}

func TestExchangeTokenRejectsUnsupportedRequestedTokenType(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read"},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
		RequestedTokenType: "urn:ietf:params:oauth:token-type:refresh_token",
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("unsupported requested_token_type が拒否されませんでした")
	}
	if issuer.calls != 0 {
		t.Fatal("拒否されたのにトークンが発行されました")
	}
}

func TestExchangeTokenUsesAuthorizerPolicyGate(t *testing.T) {
	issuer := &recordingIssuer{}
	authorizer := &denyAuthorizer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": {Active: true, Sub: "user-1", Scope: "read"},
	})
	deps.Authorizer = authorizer
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"}, Scope: "read",
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("policy denial が拒否されませんでした")
	}
	if issuer.calls != 0 {
		t.Fatal("policy denial 後にトークンが発行されました")
	}
	if authorizer.last.Action != spec.ActionTokenGrantTokenExchange {
		t.Fatalf("action=%q", authorizer.last.Action)
	}
	if authorizer.last.Context.ActorSub != "client" || authorizer.last.Context.SubjectSub != "user-1" ||
		authorizer.last.Context.Audience != "https://api.example" {
		t.Fatalf("policy context=%+v", authorizer.last.Context)
	}
	if got := authorizer.last.Resource.Properties.Scopes; len(got) != 1 || got[0] != "read" {
		t.Fatalf("policy scopes=%v", got)
	}
}

func TestExchangeTokenActorTokenBecomesActor(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := newExchangeTokenDeps(t, issuer, map[string]*ports.IntrospectionResult{
		"subj":  {Active: true, Sub: "user-1", Scope: "read"},
		"actor": {Active: true, Sub: "svc-actor", Scope: "read"},
	})
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", ActorToken: "actor",
		Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	if issuer.lastInput.Act["sub"] != "svc-actor" {
		t.Fatalf("act.sub=%v, want svc-actor (actor_token.sub)", issuer.lastInput.Act["sub"])
	}
}
