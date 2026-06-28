package policy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ra-idp-go/internal/shared/spec"
)

func TestRemoteAuthorizer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/access/v1/evaluation" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if _, ok := req["subject"]; !ok {
			t.Error("subject missing from AuthZEN request")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"decision": true})
	}))
	defer server.Close()
	remote := NewRemote(server.URL)
	result, err := remote.Authorize(t.Context(), spec.AuthZRequest{
		Subject: spec.AuthZSubject{Type: "Client", ID: "client"},
		Action:  spec.ActionTokenIntrospect,
		Resource: spec.AuthZResource{
			Type: "AccessToken",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Permit {
		t.Fatal("remote permit was not returned")
	}
}
