package spec

// Authentication bounded context の双子定義。MFA factor とログインセッション / 要求。

import "time"

type MfaFactor struct {
	Sub        string        `json:"sub"`
	Type       MfaFactorType `json:"type"`
	Secret     *string       `json:"secret,omitempty"`
	Label      *string       `json:"label,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	LastUsedAt *time.Time    `json:"last_used_at,omitempty"`
}

func (m MfaFactor) Validate() error {
	return validate(mfaFactorSchema, &m)
}

type LoginSession struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	Sub                   string    `json:"sub"`
	AuthTime              int64     `json:"auth_time"`
	AMR                   []string  `json:"amr"`
	ACR                   string    `json:"acr"`
	AuthenticationPending bool      `json:"authentication_pending"`
	ExpiresAt             time.Time `json:"expires_at"`
	// StepUpAt は直近で password / MFA による step-up 再認証が成立した時刻 (Unix 秒、
	// 未実施は 0)。高 sensitivity な self-service 操作の recency 判定に使う (ADR-043)。
	StepUpAt int64 `json:"step_up_at,omitempty"`
}

func (s LoginSession) Validate() error {
	return validate(loginSessionSchema, &s)
}

type LoginRequest struct {
	RequestID string `json:"request_id"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Csrf      string `json:"csrf"`
}

func (r LoginRequest) Validate() error {
	return validate(loginRequestSchema, &r)
}
