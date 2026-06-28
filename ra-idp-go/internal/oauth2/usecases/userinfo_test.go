package usecases_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
)

func strptr(s string) *string { return &s }

func userInfoFixture(t *testing.T) *memory.UserRepository {
	t.Helper()
	repo := memory.NewUserRepository()
	repo.Seed(&spec.User{
		Sub: "user-1", TenantID: spec.DefaultTenantID, PreferredUsername: "carol",
		Name: strptr("Carol Q"), Email: strptr("carol@example.com"), EmailVerified: true,
		Lifecycle: spec.UserLifecycle{Status: spec.UserStatusActive},
		Attributes: map[string]spec.AttributeValue{
			"nickname":     {Type: spec.AttributeTypeString, String: strptr("cici")},
			"phone_number": {Type: spec.AttributeTypeString, String: strptr("+819012345678")},
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	return repo
}

func resolveBuiltin(_ context.Context, _ string) ([]spec.UserAttributeDef, error) {
	return spec.BuiltinUserAttributeDefs(), nil
}

func TestUserInfoIncludesAttributeClaimsByScope(t *testing.T) {
	repo := userInfoFixture(t)
	res, err := usecases.UserInfo(context.Background(), repo, nil, usecases.UserInfoInput{
		Scopes: []string{"openid", "profile", "phone"}, Sub: "user-1", Active: true, ClientID: "c1",
		ResolveAttributeDefs: resolveBuiltin,
	})
	if err != nil {
		t.Fatal(err)
	}
	// MarshalJSON が標準 claim と属性 claim をマージすることを確認する。
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "Carol Q" {
		t.Fatalf("standard claim missing: %#v", got)
	}
	if got["nickname"] != "cici" {
		t.Fatalf("nickname claim missing: %#v", got)
	}
	if got["phone_number"] != "+819012345678" {
		t.Fatalf("phone_number claim missing: %#v", got)
	}
}

func TestUserInfoOmitsAttributeClaimsWithoutScope(t *testing.T) {
	repo := userInfoFixture(t)
	res, err := usecases.UserInfo(context.Background(), repo, nil, usecases.UserInfoInput{
		Scopes: []string{"openid"}, Sub: "user-1", Active: true, ClientID: "c1",
		ResolveAttributeDefs: resolveBuiltin,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["nickname"]; ok {
		t.Fatalf("nickname leaked without profile scope: %#v", got)
	}
	if _, ok := got["phone_number"]; ok {
		t.Fatalf("phone_number leaked without phone scope: %#v", got)
	}
}
