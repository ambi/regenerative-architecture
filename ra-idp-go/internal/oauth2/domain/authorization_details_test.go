package domain

import (
	"testing"

	"ra-idp-go/internal/spec"
)

// paymentType は RFC 9396 の payment_initiation を模した登録 type。
// actions は集合包含、creditorAccount は enum、instructedAmount は上限 (単調減少)。
func paymentType() spec.AuthorizationDetailType {
	return spec.AuthorizationDetailType{
		TenantID: "default",
		Type:     "payment_initiation",
		State:    spec.DetailTypeEnabled,
		Schema: spec.AuthorizationDetailsSchema{
			Rules: []spec.AuthorizationDetailFieldRule{
				{Name: "actions", Semantics: spec.DetailFieldSet, Required: true, Allowed: []string{"initiate", "status", "cancel"}},
				{Name: "creditorAccount", Semantics: spec.DetailFieldEnum, Allowed: []string{"acct-x", "acct-y"}},
				{Name: "instructedAmount", Semantics: spec.DetailFieldAtMost, Required: true},
			},
		},
		DisplayTemplate: "{creditorAccount} へ最大 {instructedAmount} まで",
	}
}

func detail(fields map[string]any, actions ...string) spec.AuthorizationDetail {
	return spec.AuthorizationDetail{Type: "payment_initiation", Actions: actions, Fields: fields}
}

func TestValidateAgainstType_Valid(t *testing.T) {
	d := detail(map[string]any{"instructedAmount": float64(100), "creditorAccount": "acct-x"}, "initiate")
	if err := ValidateAgainstType(d, paymentType()); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateAgainstType_FailClosed(t *testing.T) {
	cases := map[string]spec.AuthorizationDetail{
		"unregistered field":   {Type: "payment_initiation", Actions: []string{"initiate"}, Identifier: "x", Fields: map[string]any{"instructedAmount": float64(10)}},
		"missing required":     {Type: "payment_initiation", Actions: []string{"initiate"}},                                      // instructedAmount 欠落
		"action not allowed":   detail(map[string]any{"instructedAmount": float64(10)}, "wire-transfer"),                         // allowed 外
		"enum not allowed":     detail(map[string]any{"instructedAmount": float64(10), "creditorAccount": "acct-z"}, "initiate"), // enum allowed 外
		"amount not number":    detail(map[string]any{"instructedAmount": "lots"}, "initiate"),                                   // at_most が数値でない
		"enum multiple values": {Type: "payment_initiation", Actions: []string{"initiate"}, Fields: map[string]any{"instructedAmount": float64(10), "creditorAccount": []any{"acct-x", "acct-y"}}},
		"type mismatch":        {Type: "data_access", Actions: []string{"initiate"}, Fields: map[string]any{"instructedAmount": float64(10)}},
	}
	for name, d := range cases {
		if err := ValidateAgainstType(d, paymentType()); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

func TestValidateAgainstType_DisabledRejected(t *testing.T) {
	pt := paymentType()
	pt.State = spec.DetailTypeDisabled
	d := detail(map[string]any{"instructedAmount": float64(10)}, "initiate")
	if err := ValidateAgainstType(d, pt); err == nil {
		t.Fatal("expected rejection for disabled type")
	}
}

func TestDetailsSubsetOf_Downscope(t *testing.T) {
	types := map[string]spec.AuthorizationDetailType{"payment_initiation": paymentType()}
	granted := []spec.AuthorizationDetail{
		detail(map[string]any{"instructedAmount": float64(100), "creditorAccount": "acct-x"}, "initiate", "status"),
	}

	// 部分集合: 操作を減らし、金額を下げる → 許容。
	ok := []spec.AuthorizationDetail{detail(map[string]any{"instructedAmount": float64(50), "creditorAccount": "acct-x"}, "initiate")}
	if err := DetailsSubsetOf(ok, granted, types); err != nil {
		t.Fatalf("expected subset, got %v", err)
	}

	// 拡大ケースはすべて拒否されねばならない (fail-closed)。
	exceed := map[string][]spec.AuthorizationDetail{
		"amount over":       {detail(map[string]any{"instructedAmount": float64(150), "creditorAccount": "acct-x"}, "initiate")},
		"action not in set": {detail(map[string]any{"instructedAmount": float64(10), "creditorAccount": "acct-x"}, "cancel")},
		"creditor changed":  {detail(map[string]any{"instructedAmount": float64(10), "creditorAccount": "acct-y"}, "initiate")},
		"unregistered type": {{Type: "data_access"}},
	}
	for name, req := range exceed {
		if err := DetailsSubsetOf(req, granted, types); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

func TestDetailsSubsetOf_OmittedFieldNarrows(t *testing.T) {
	types := map[string]spec.AuthorizationDetailType{"payment_initiation": paymentType()}
	granted := []spec.AuthorizationDetail{
		detail(map[string]any{"instructedAmount": float64(100), "creditorAccount": "acct-x"}, "initiate", "status"),
	}
	// creditorAccount を省く要求は縛りを緩める方向だが、それでも基準を超えないので許容。
	req := []spec.AuthorizationDetail{detail(map[string]any{"instructedAmount": float64(20)}, "initiate")}
	if err := DetailsSubsetOf(req, granted, types); err != nil {
		t.Fatalf("expected subset when field omitted, got %v", err)
	}
}

func TestDetailTypes_Dedup(t *testing.T) {
	got := DetailTypes([]spec.AuthorizationDetail{{Type: "a"}, {Type: "b"}, {Type: "a"}})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected dedup result: %v", got)
	}
}
