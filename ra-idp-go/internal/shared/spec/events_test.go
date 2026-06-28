package spec

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMarshalDomainEventUsesContractFieldNames(t *testing.T) {
	data, err := MarshalDomainEvent(&RefreshTokenIssued{
		At:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TokenID: "token", FamilyID: "family", ClientID: "client", Sub: "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["type"] != "RefreshTokenIssued" || wire["familyId"] != "family" {
		t.Fatalf("unexpected event wire: %s", data)
	}
	if _, exists := wire["FamilyID"]; exists {
		t.Fatalf("Go field name leaked: %s", data)
	}
}
