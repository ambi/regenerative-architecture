package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"
)

func seedPaymentType(repo *memory.AuthorizationDetailTypeRepository) {
	repo.Seed(&spec.AuthorizationDetailType{
		TenantID: spec.DefaultTenantID, Type: "payment_initiation", State: spec.DetailTypeEnabled,
		Schema: spec.AuthorizationDetailsSchema{Rules: []spec.AuthorizationDetailFieldRule{
			{Name: "actions", Semantics: spec.DetailFieldSet, Required: true, Allowed: []string{"initiate", "status"}},
			{Name: "instructedAmount", Semantics: spec.DetailFieldAtMost, Required: true},
		}},
		DisplayTemplate: "最大 {instructedAmount} まで", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
}

func authorizeDepsWithTypes() AuthorizeDeps {
	deps := newAuthorizeDeps(false)
	types := memory.NewAuthorizationDetailTypeRepository()
	seedPaymentType(types)
	deps.AuthzDetailTypeRepo = types
	return deps
}

func paymentDetail(amount float64, actions ...string) spec.AuthorizationDetail {
	return spec.AuthorizationDetail{Type: "payment_initiation", Actions: actions, Fields: map[string]any{"instructedAmount": amount}}
}

func TestAuthorizeStoresValidAuthorizationDetails(t *testing.T) {
	deps := authorizeDepsWithTypes()
	in := validAuthorizeInput()
	in.AuthorizationDetails = []spec.AuthorizationDetail{paymentDetail(100, "initiate")}
	out, err := Authorize(context.Background(), deps, in)
	if err != nil {
		t.Fatalf("expected valid details accepted, got %v", err)
	}
	if len(out.Request.AuthorizationDetails) != 1 {
		t.Fatalf("expected details stored on request, got %d", len(out.Request.AuthorizationDetails))
	}
}

func TestAuthorizeRejectsUnregisteredDetailType(t *testing.T) {
	deps := authorizeDepsWithTypes()
	in := validAuthorizeInput()
	in.AuthorizationDetails = []spec.AuthorizationDetail{{Type: "data_access"}}
	if _, err := Authorize(context.Background(), deps, in); err == nil {
		t.Fatal("expected rejection for unregistered type")
	}
}

func TestAuthorizeRejectsSchemaViolation(t *testing.T) {
	deps := authorizeDepsWithTypes()
	in := validAuthorizeInput()
	// instructedAmount (required) を欠く → fail-closed。
	in.AuthorizationDetails = []spec.AuthorizationDetail{{Type: "payment_initiation", Actions: []string{"initiate"}}}
	if _, err := Authorize(context.Background(), deps, in); err == nil {
		t.Fatal("expected rejection for schema violation")
	}
}

func TestAuthorizeRejectsDetailsWhenRegistryAbsent(t *testing.T) {
	// レジストリ未配線で details を要求したら受理しない (fail-closed)。
	deps := newAuthorizeDeps(false)
	in := validAuthorizeInput()
	in.AuthorizationDetails = []spec.AuthorizationDetail{paymentDetail(100, "initiate")}
	if _, err := Authorize(context.Background(), deps, in); err == nil {
		t.Fatal("expected rejection when registry is absent")
	}
}

func TestParseAuthorizationDetailsRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseAuthorizationDetails("{not-an-array"); err == nil {
		t.Fatal("expected malformed JSON rejection")
	}
	got, err := ParseAuthorizationDetails("")
	if err != nil || got != nil {
		t.Fatalf("expected empty input to yield nil, nil; got %v %v", got, err)
	}
}
