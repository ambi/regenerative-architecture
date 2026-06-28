package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/shared/spec"
)

type Local struct{}

func (Local) Authorize(_ context.Context, req spec.AuthZRequest) (spec.AuthZResponse, error) {
	return spec.Evaluate(req), nil
}

type Remote struct {
	Endpoint string
	Client   *http.Client
}

func NewRemote(endpoint string) *Remote {
	return &Remote{
		Endpoint: strings.TrimRight(endpoint, "/") + "/access/v1/evaluation",
		Client:   &http.Client{Timeout: 2 * time.Second},
	}
}

func (r *Remote) Authorize(ctx context.Context, input spec.AuthZRequest) (spec.AuthZResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return spec.AuthZResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(body))
	if err != nil {
		return spec.AuthZResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		return spec.AuthZResponse{}, fmt.Errorf("authzen request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return spec.AuthZResponse{}, fmt.Errorf("authzen response %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	var wire struct {
		Decision bool     `json:"decision"`
		Permit   *bool    `json:"permit"`
		Reasons  []string `json:"reasons"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&wire); err != nil {
		return spec.AuthZResponse{}, fmt.Errorf("authzen decode: %w", err)
	}
	permit := wire.Decision
	if wire.Permit != nil {
		permit = *wire.Permit
	}
	if !permit && len(wire.Reasons) == 0 {
		wire.Reasons = []string{"remote_policy_denied"}
	}
	if permit && len(wire.Reasons) > 0 {
		return spec.AuthZResponse{}, errors.New("authzen returned permit with denial reasons")
	}
	return spec.AuthZResponse{Permit: permit, Reasons: wire.Reasons}, nil
}
