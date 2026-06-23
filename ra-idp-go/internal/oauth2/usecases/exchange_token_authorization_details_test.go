package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"
)

func exchangeDepsWithDetailTypes(t *testing.T, issuer *recordingIssuer, results map[string]*ports.IntrospectionResult) ExchangeTokenDeps {
	t.Helper()
	deps := newExchangeTokenDeps(t, issuer, results)
	types := memory.NewAuthorizationDetailTypeRepository()
	seedPaymentType(types)
	deps.AuthzDetailTypeRepo = types
	return deps
}

func subjectWithDetails(amount float64, actions ...string) *ports.IntrospectionResult {
	return &ports.IntrospectionResult{
		Active: true, Sub: "user-1", Scope: "read write",
		AuthorizationDetails: []spec.AuthorizationDetail{paymentDetail(amount, actions...)},
	}
}

func TestExchangeTokenPreservesSubjectDetailsByDefault(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := exchangeDepsWithDetailTypes(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": subjectWithDetails(100, "initiate", "status"),
	})
	res, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	if len(res.AuthorizationDetails) != 1 || len(issuer.lastInput.AuthorizationDetails) != 1 {
		t.Fatalf("expected subject details preserved, got result=%v issued=%v", res.AuthorizationDetails, issuer.lastInput.AuthorizationDetails)
	}
}

func TestExchangeTokenDownscopesDetails(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := exchangeDepsWithDetailTypes(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": subjectWithDetails(100, "initiate", "status"),
	})
	// 金額を下げ操作を絞る → 部分集合、許容。
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
		AuthorizationDetails: []spec.AuthorizationDetail{paymentDetail(40, "initiate")},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	got := issuer.lastInput.AuthorizationDetails
	if len(got) != 1 || got[0].Fields["instructedAmount"] != float64(40) {
		t.Fatalf("expected downscoped detail amount=40, got %v", got)
	}
}

func TestExchangeTokenRejectsDetailExpansion(t *testing.T) {
	issuer := &recordingIssuer{}
	deps := exchangeDepsWithDetailTypes(t, issuer, map[string]*ports.IntrospectionResult{
		"subj": subjectWithDetails(100, "initiate"),
	})
	// 金額を上げる要求は subject token の詳細を超える → 拒否 (fail-closed)。
	_, err := ExchangeToken(context.Background(), deps, ExchangeTokenInput{
		ClientID: "client", SubjectToken: "subj", Resource: []string{"https://api.example"},
		AuthorizationDetails: []spec.AuthorizationDetail{paymentDetail(500, "initiate")},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("expected rejection for detail expansion")
	}
	if issuer.calls != 0 {
		t.Fatalf("token must not be issued on rejected exchange, calls=%d", issuer.calls)
	}
}
