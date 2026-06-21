package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/platform/persistence/memory"
)

type staticIntrospector struct {
	result *ports.IntrospectionResult
}

func (s staticIntrospector) IntrospectAccessToken(
	context.Context,
	string,
) (*ports.IntrospectionResult, error) {
	return s.result, nil
}

func TestRevokeAccessTokenAddsOwnedJTIToDenylist(t *testing.T) {
	ctx := context.Background()
	denylist := memory.NewAccessTokenDenylist()
	expiresAt := time.Now().Add(time.Minute)
	err := RevokeToken(ctx, RevokeDeps{
		RefreshStore: memory.NewRefreshTokenStore(),
		Introspector: staticIntrospector{result: &ports.IntrospectionResult{
			Active: true, JTI: "jti-1", ClientID: "client", Exp: expiresAt.Unix(),
		}},
		AccessTokenDenylist: denylist,
	}, "client", "header.payload.signature", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := denylist.IsRevoked(ctx, "jti-1")
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("access token jti was not denylisted")
	}
}

func TestRevokeAccessTokenIgnoresOtherClient(t *testing.T) {
	ctx := context.Background()
	denylist := memory.NewAccessTokenDenylist()
	err := RevokeToken(ctx, RevokeDeps{
		RefreshStore: memory.NewRefreshTokenStore(),
		Introspector: staticIntrospector{result: &ports.IntrospectionResult{
			Active: true, JTI: "jti-1", ClientID: "owner", Exp: time.Now().Add(time.Minute).Unix(),
		}},
		AccessTokenDenylist: denylist,
	}, "attacker", "header.payload.signature", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := denylist.IsRevoked(ctx, "jti-1")
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("another client's access token was revoked")
	}
}
