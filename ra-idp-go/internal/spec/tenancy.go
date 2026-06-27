package spec

// Tenancy bounded context の双子定義 (ADR-032 / ADR-034)。

import "time"

const DefaultTenantID = "default"

type Tenant struct {
	ID                     string                  `json:"id"`
	DisplayName            string                  `json:"display_name"`
	Status                 TenantStatus            `json:"status"`
	PasswordPolicyOverride *PasswordPolicyOverride `json:"password_policy_override,omitempty"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              *time.Time              `json:"updated_at,omitempty"`
	DisabledAt             *time.Time              `json:"disabled_at,omitempty"`
}

func (t Tenant) Validate() error {
	return validate(tenantSchema, &t)
}

// PasswordPolicyOverride はテナント固有の objectives.PasswordPolicy 上書き値。
// SCL `PasswordPolicyOverride` の双子定義。省略フィールドは global default を継承する。
type PasswordPolicyOverride struct {
	MinLength    *int `json:"min_length,omitempty"`
	MaxLength    *int `json:"max_length,omitempty"`
	HistoryDepth *int `json:"history_depth,omitempty"`
}
