package spec

// IdentityManagement bounded context の User / 属性モデル (ADR-039 / ADR-040)。
// 組み込み属性カタログと claim 射影は attributes.go が持つ。

import (
	"fmt"
	"time"
)

// User は IdP の最小コア。識別・認証・表示名・RBAC・ライフサイクルだけを型付きで
// 持ち、それ以外のプロフィール属性 (OIDC §5.1 optional claim / 組織属性 / tenant
// 定義 custom) は Attributes に sparse に格納する (ADR-039)。テナントは実際に使う
// 属性しか持たないため、固定カラム/フィールドの肥大を避けられる。
type User struct {
	Sub               string                    `json:"sub"`
	TenantID          string                    `json:"tenant_id"`
	PreferredUsername string                    `json:"preferred_username"`
	PasswordHash      string                    `json:"password_hash"`
	Name              *string                   `json:"name,omitempty"`
	GivenName         *string                   `json:"given_name,omitempty"`
	FamilyName        *string                   `json:"family_name,omitempty"`
	Email             *string                   `json:"email,omitempty"`
	EmailVerified     bool                      `json:"email_verified"`
	MfaEnrolled       bool                      `json:"mfa_enrolled"`
	Roles             []string                  `json:"roles"`
	Lifecycle         UserLifecycle             `json:"lifecycle"`
	Attributes        map[string]AttributeValue `json:"attributes,omitempty"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

func (u User) Validate() error {
	if err := validate(userSchema, &u); err != nil {
		return err
	}
	if err := u.Lifecycle.Validate(); err != nil {
		return err
	}
	for key, v := range u.Attributes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
	}
	return nil
}

// IsDeleted は User が ADR-036 の tombstone 状態 (status == Deleted) かを返す。
func (u User) IsDeleted() bool { return u.Lifecycle.EffectiveStatus() == UserStatusDeleted }

// IsSoftDeleted は User が削除予約状態 (status == PendingDeletion) かを返す。PII は
// 温存されるが認証は Disabled / Deleted と同じく拒否される (IsActive が false)。
func (u User) IsSoftDeleted() bool {
	return u.Lifecycle.EffectiveStatus() == UserStatusPendingDeletion
}

// IsActive は認証を許可してよい状態 (status == Active) かを返す。
// Disabled / Locked / Staged / Suspended / Deleted はすべて非アクティブ。
func (u User) IsActive() bool { return u.Lifecycle.EffectiveStatus() == UserStatusActive }

// UserLifecycle は User の運用ライフサイクル属性。status は状態機械 UserLifecycle
// (states セクション) と一致する唯一の真実。status_changed_at が現在の状態に
// なった時刻 (旧 disabled_at / deleted_at を統合)。
type UserLifecycle struct {
	Status            UserStatus       `json:"status"`
	StatusChangedAt   *time.Time       `json:"status_changed_at,omitempty"`
	LastLoginAt       *time.Time       `json:"last_login_at,omitempty"`
	PasswordChangedAt *time.Time       `json:"password_changed_at,omitempty"`
	RequiredActions   []RequiredAction `json:"required_actions,omitempty"`
}

// EffectiveStatus は未設定 (zero-value) を既定の Active として解決する。
// SCL の `status: { default: Active }` と整合し、新規構築 User を Active 扱いにする。
func (l UserLifecycle) EffectiveStatus() UserStatus {
	if l.Status == "" {
		return UserStatusActive
	}
	return l.Status
}

func (l UserLifecycle) Validate() error {
	if l.Status != "" && !l.Status.Valid() {
		return fmt.Errorf("user status %q is not in enum", l.Status)
	}
	for _, a := range l.RequiredActions {
		if !a.Valid() {
			return fmt.Errorf("required action %q is not in enum", a)
		}
	}
	return nil
}

// AttributeValue は属性 1 件の値 (sum type)。Type が示すフィールドだけが設定される。
// OIDC 標準クレームの組み込み属性と tenant 定義 custom 属性で共通 (ADR-040)。
type AttributeValue struct {
	Type        AttributeType `json:"type"`
	String      *string       `json:"string,omitempty"`
	Number      *float64      `json:"number,omitempty"`
	Boolean     *bool         `json:"boolean,omitempty"`
	Date        *string       `json:"date,omitempty"` // ISO 8601 date
	StringArray []string      `json:"string_array,omitempty"`
}

func (v AttributeValue) Validate() error {
	if !v.Type.Valid() {
		return fmt.Errorf("attribute type %q is not in enum", v.Type)
	}
	set := 0
	matches := false
	check := func(present bool, t AttributeType) {
		if present {
			set++
			if v.Type == t {
				matches = true
			}
		}
	}
	check(v.String != nil, AttributeTypeString)
	check(v.Number != nil, AttributeTypeNumber)
	check(v.Boolean != nil, AttributeTypeBoolean)
	check(v.Date != nil, AttributeTypeDate)
	check(v.StringArray != nil, AttributeTypeStringArray)
	if set != 1 || !matches {
		return fmt.Errorf("attribute value must set exactly the one field matching type %q", v.Type)
	}
	return nil
}

// JSONValue は属性値を OIDC claim へ載せる JSON ネイティブ値に変換する。型と
// 中身が食い違う / 値が無い場合は nil を返し、呼び出し側が claim を省略できる。
func (v AttributeValue) JSONValue() any {
	switch v.Type {
	case AttributeTypeString:
		if v.String != nil {
			return *v.String
		}
	case AttributeTypeNumber:
		if v.Number != nil {
			return *v.Number
		}
	case AttributeTypeBoolean:
		if v.Boolean != nil {
			return *v.Boolean
		}
	case AttributeTypeDate:
		if v.Date != nil {
			return *v.Date
		}
	case AttributeTypeStringArray:
		if v.StringArray != nil {
			return v.StringArray
		}
	}
	return nil
}

// UserAttributeDef は属性 1 件の定義 (ADR-040)。OIDC 組み込みカタログ
// (BuiltinUserAttributeDefs) と tenant 定義 (TenantUserAttributeSchema) の両方で使う。
type UserAttributeDef struct {
	Key            string         `json:"key"`
	Label          string         `json:"label,omitempty"` // 利用者向けの日本語表示名 (任意)
	Type           AttributeType  `json:"type"`
	MultiValued    bool           `json:"multi_valued"`
	Required       bool           `json:"required"`
	EditableByUser bool           `json:"editable_by_user"`
	ClaimName      *string        `json:"claim_name,omitempty"` // OIDC claim 名 (露出時)
	OIDCScope      *string        `json:"oidc_scope,omitempty"` // 露出を解禁する OIDC scope
	Visibility     AttrVisibility `json:"visibility"`
	PII            bool           `json:"pii"` // 省略時は PII 扱い (hash 化) が安全側 default
}

func (d UserAttributeDef) Validate() error { return validate(userAttributeDefSchema, &d) }

// TenantUserAttributeSchema は tenant 単位の custom 属性定義集合 (ADR-040)。
// 組み込み属性は BuiltinUserAttributeDefs() がコードで持ち、本集合は tenant 固有分のみ。
// tenant 削除時に cascade する。
type TenantUserAttributeSchema struct {
	TenantID   string             `json:"tenant_id"`
	Attributes []UserAttributeDef `json:"attributes"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

func (s TenantUserAttributeSchema) Validate() error {
	builtin := map[string]bool{}
	for _, d := range BuiltinUserAttributeDefs() {
		builtin[d.Key] = true
	}
	seen := map[string]bool{}
	for _, d := range s.Attributes {
		if err := d.Validate(); err != nil {
			return err
		}
		if builtin[d.Key] {
			return fmt.Errorf("custom attribute %q collides with a builtin attribute", d.Key)
		}
		if seen[d.Key] {
			return fmt.Errorf("duplicate custom attribute key %q", d.Key)
		}
		seen[d.Key] = true
	}
	return nil
}

// EffectiveDefs は組み込み属性 + tenant custom 属性を結合した実効定義を返す。
func (s TenantUserAttributeSchema) EffectiveDefs() []UserAttributeDef {
	defs := BuiltinUserAttributeDefs()
	return append(defs, s.Attributes...)
}

// ValidateAttributeValue は属性値 1 件を定義に対して検証する (required は見ない)。
// 値自体の整合・型の一致・multi_valued の整合を確認する。
func ValidateAttributeValue(value AttributeValue, def UserAttributeDef) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if value.Type != def.Type {
		return fmt.Errorf("expects type %q, got %q", def.Type, value.Type)
	}
	if def.MultiValued != (def.Type == AttributeTypeStringArray) {
		return fmt.Errorf("multi_valued/type mismatch")
	}
	return nil
}

// ValidateAttributes は User.Attributes を実効属性定義に対して検証する。
// 未定義 key の拒否、型の一致、multi_valued の整合、required の充足を見る。
func ValidateAttributes(values map[string]AttributeValue, defs []UserAttributeDef) error {
	byKey := make(map[string]UserAttributeDef, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	for key, v := range values {
		def, ok := byKey[key]
		if !ok {
			return fmt.Errorf("attribute %q is not defined", key)
		}
		if err := ValidateAttributeValue(v, def); err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
	}
	for _, def := range defs {
		if def.Required {
			if _, ok := values[def.Key]; !ok {
				return fmt.Errorf("required attribute %q is missing", def.Key)
			}
		}
	}
	return nil
}
