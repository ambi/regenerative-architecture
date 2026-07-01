package spec

import "time"

// Application の双子定義 (wi-69, ADR-064)。
//
// Application は運用者が「接続する業務アプリケーション」として扱う上位 aggregate。
// OIDC client / SAML SP / WS-Fed RP を protocol binding として束ね、表示名・アイコン・
// 状態・割当を所有する。protocol binding の wire 挙動は各 protocol context が所有し、
// ここでは binding を opaque key (client_id / wtrealm) で参照するに留める。

// ApplicationKind は Application が federation を束ねるか、SSO しない外部リンクかを表す。
type ApplicationKind string

const (
	// ApplicationFederated は protocol binding を束ねる SSO アプリケーション。
	ApplicationFederated ApplicationKind = "federated"
	// ApplicationWeblink は binding を持たない外部リンク (SSO 非対応)。
	ApplicationWeblink ApplicationKind = "weblink"
	// ApplicationService は M2M / サービス用の OAuth2 client (client_credentials)。
	// ポータルタイルや利用者割当を持たず、Okta の "API Services" に相当する。
	ApplicationService ApplicationKind = "service"
)

func (k ApplicationKind) Valid() bool {
	switch k {
	case ApplicationFederated, ApplicationWeblink, ApplicationService:
		return true
	}
	return false
}

// ApplicationStatus は Application の有効状態。
type ApplicationStatus string

const (
	ApplicationActive   ApplicationStatus = "active"
	ApplicationDisabled ApplicationStatus = "disabled"
)

func (s ApplicationStatus) Valid() bool {
	switch s {
	case ApplicationActive, ApplicationDisabled:
		return true
	}
	return false
}

// ProtocolBindingType は Application に接続する binding の種別 (ADR-064)。
type ProtocolBindingType string

const (
	ProtocolBindingOIDC  ProtocolBindingType = "oidc"
	ProtocolBindingSAML  ProtocolBindingType = "saml"
	ProtocolBindingWsFed ProtocolBindingType = "wsfed"
)

func (t ProtocolBindingType) Valid() bool {
	switch t {
	case ProtocolBindingOIDC, ProtocolBindingSAML, ProtocolBindingWsFed:
		return true
	}
	return false
}

// AssignmentSubjectType は割当対象が user か group かを表す。
type AssignmentSubjectType string

const (
	AssignmentSubjectUser  AssignmentSubjectType = "user"
	AssignmentSubjectGroup AssignmentSubjectType = "group"
)

func (t AssignmentSubjectType) Valid() bool {
	switch t {
	case AssignmentSubjectUser, AssignmentSubjectGroup:
		return true
	}
	return false
}

// AssignmentVisibility は割当の可視性。hidden はポータル一覧から除外するが
// プロトコル利用は引き続き許可する (wi-69)。
type AssignmentVisibility string

const (
	AssignmentVisible AssignmentVisibility = "visible"
	AssignmentHidden  AssignmentVisibility = "hidden"
)

func (v AssignmentVisibility) Valid() bool {
	switch v {
	case AssignmentVisible, AssignmentHidden:
		return true
	}
	return false
}

// ProtocolBinding は Application に紐づく protocol binding (wi-69, ADR-064)。
// binding key は protocol ごとに異なる: OIDC は client_id、WS-Fed は wtrealm、SAML は entity_id。
type ProtocolBinding struct {
	Type     ProtocolBindingType `json:"type"`
	ClientID string              `json:"client_id,omitempty"`
	Wtrealm  string              `json:"wtrealm,omitempty"`
	EntityID string              `json:"entity_id,omitempty"`
}

// Application は運用者向けの上位 aggregate (wi-69)。
type Application struct {
	TenantID      string            `json:"tenant_id"`
	ApplicationID string            `json:"application_id"`
	Name          string            `json:"name"`
	Kind          ApplicationKind   `json:"kind"`
	Status        ApplicationStatus `json:"status"`
	IconURL       string            `json:"icon_url,omitempty"`
	IconObjectKey string            `json:"icon_object_key,omitempty"`
	LaunchURL     string            `json:"launch_url,omitempty"`
	Bindings      []ProtocolBinding `json:"bindings"`
	CategoryIDs   []string          `json:"category_ids"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ApplicationIcon は tenant-scoped に保存された Application アイコン画像。
type ApplicationIcon struct {
	TenantID      string    `json:"tenant_id"`
	ApplicationID string    `json:"application_id"`
	ObjectKey     string    `json:"object_key"`
	ContentType   string    `json:"content_type"`
	SizeBytes     int       `json:"size_bytes"`
	Data          []byte    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

// ApplicationCategory は管理者が tenant 単位で定義するポータルの分類セクション (wi-70, ADR-069)。
// Application に 0..N 個付与され、利用者ポータルはこのカテゴリ単位でタイルをセクション表示する。
type ApplicationCategory struct {
	TenantID   string    `json:"tenant_id"`
	CategoryID string    `json:"category_id"`
	Name       string    `json:"name"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ApplicationAssignment は Application へのユーザー / グループ割当 (wi-69)。
type ApplicationAssignment struct {
	TenantID      string                `json:"tenant_id"`
	ApplicationID string                `json:"application_id"`
	SubjectType   AssignmentSubjectType `json:"subject_type"`
	SubjectID     string                `json:"subject_id"`
	Visibility    AssignmentVisibility  `json:"visibility"`
	CreatedAt     time.Time             `json:"created_at"`
}

// ApplicationOrdering は利用者ポータルでの手動並び順 (wi-70, ADR-069)。
// tenant_id + user_sub をキーに、Application の表示順を application_id の順序列で持つ。
type ApplicationOrdering struct {
	TenantID       string    `json:"tenant_id"`
	UserSub        string    `json:"user_sub"`
	ApplicationIDs []string  `json:"application_ids"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ApplicationCreated は Application を作成した event (wi-69)。
type ApplicationCreated struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
}

func (e *ApplicationCreated) EventType() string     { return "ApplicationCreated" }
func (e *ApplicationCreated) OccurredAt() time.Time { return e.At }

// ApplicationUpdated は Application のメタデータを更新した event (wi-69)。
type ApplicationUpdated struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
	ChangedFields []string  `json:"changedFields"`
}

func (e *ApplicationUpdated) EventType() string     { return "ApplicationUpdated" }
func (e *ApplicationUpdated) OccurredAt() time.Time { return e.At }

// ApplicationIconUpdated は Application の保存済みアイコンを更新した event。
type ApplicationIconUpdated struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
	Action        string    `json:"action"`
}

func (e *ApplicationIconUpdated) EventType() string     { return "ApplicationIconUpdated" }
func (e *ApplicationIconUpdated) OccurredAt() time.Time { return e.At }

// ApplicationDeleted は Application を削除した event (wi-69)。
type ApplicationDeleted struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
}

func (e *ApplicationDeleted) EventType() string     { return "ApplicationDeleted" }
func (e *ApplicationDeleted) OccurredAt() time.Time { return e.At }

// ProtocolBindingAttached は Application に protocol binding を接続した event (wi-69)。
type ProtocolBindingAttached struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
	BindingType   string    `json:"bindingType"`
}

func (e *ProtocolBindingAttached) EventType() string     { return "ProtocolBindingAttached" }
func (e *ProtocolBindingAttached) OccurredAt() time.Time { return e.At }

// ProtocolBindingDetached は Application から protocol binding を解除した event (wi-69)。
type ProtocolBindingDetached struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
	BindingType   string    `json:"bindingType"`
}

func (e *ProtocolBindingDetached) EventType() string     { return "ProtocolBindingDetached" }
func (e *ProtocolBindingDetached) OccurredAt() time.Time { return e.At }

// ApplicationAssigned は Application にユーザー / グループを割当てた event (wi-69)。
type ApplicationAssigned struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
	SubjectType   string    `json:"subjectType"`
	SubjectID     string    `json:"subjectId"`
}

func (e *ApplicationAssigned) EventType() string     { return "ApplicationAssigned" }
func (e *ApplicationAssigned) OccurredAt() time.Time { return e.At }

// ApplicationUnassigned は Application の割当を解除した event (wi-69)。
type ApplicationUnassigned struct {
	At            time.Time `json:"-"`
	TenantID      string    `json:"tenantId"`
	ActorSub      string    `json:"actorSub"`
	ApplicationID string    `json:"applicationId"`
	SubjectType   string    `json:"subjectType"`
	SubjectID     string    `json:"subjectId"`
}

func (e *ApplicationUnassigned) EventType() string     { return "ApplicationUnassigned" }
func (e *ApplicationUnassigned) OccurredAt() time.Time { return e.At }

// ApplicationCategoryCreated は ApplicationCategory を作成した event (wi-70)。
type ApplicationCategoryCreated struct {
	At         time.Time `json:"-"`
	TenantID   string    `json:"tenantId"`
	ActorSub   string    `json:"actorSub"`
	CategoryID string    `json:"categoryId"`
}

func (e *ApplicationCategoryCreated) EventType() string     { return "ApplicationCategoryCreated" }
func (e *ApplicationCategoryCreated) OccurredAt() time.Time { return e.At }

// ApplicationCategoryUpdated は ApplicationCategory を更新した event (wi-70)。
type ApplicationCategoryUpdated struct {
	At         time.Time `json:"-"`
	TenantID   string    `json:"tenantId"`
	ActorSub   string    `json:"actorSub"`
	CategoryID string    `json:"categoryId"`
}

func (e *ApplicationCategoryUpdated) EventType() string     { return "ApplicationCategoryUpdated" }
func (e *ApplicationCategoryUpdated) OccurredAt() time.Time { return e.At }

// ApplicationCategoryDeleted は ApplicationCategory を削除した event (wi-70)。
type ApplicationCategoryDeleted struct {
	At         time.Time `json:"-"`
	TenantID   string    `json:"tenantId"`
	ActorSub   string    `json:"actorSub"`
	CategoryID string    `json:"categoryId"`
}

func (e *ApplicationCategoryDeleted) EventType() string     { return "ApplicationCategoryDeleted" }
func (e *ApplicationCategoryDeleted) OccurredAt() time.Time { return e.At }
