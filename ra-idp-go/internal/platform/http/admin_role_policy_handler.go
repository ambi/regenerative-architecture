package http

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// adrReference は SCL の説明文に埋め込まれた ADR 参照 (例 " (ADR-031 / ADR-032)")。
// 設計ドキュメントの語彙なので、管理者向けレスポンスからは取り除く。
var adrReference = regexp.MustCompile(`\s*\(ADR-[^)]*\)`)

// sanitizeAdminCopy は SCL 由来の説明文を管理 UI 向けに整える。内部の ADR 参照を
// 取り除くだけで文意は保つ。allow_when 式 (requirements) は別途レスポンスに含めない。
func sanitizeAdminCopy(text string) string {
	return strings.TrimSpace(adrReference.ReplaceAllString(text, ""))
}

// roleDescriptionCopy / permissionDescriptionCopy は利用者向けの平易な説明。
// SCL の normative 定義は設計者向け語彙 (User.roles / SystemAdministrator /
// tombstone 等) を含むため、管理 UI には用語集を引いた user-facing コピーを返す。
// 未登録の名前は SCL 由来の説明を ADR 除去のうえフォールバックする。
var roleDescriptionCopy = map[string]string{
	"admin":        "所属テナント内のユーザー・アプリケーション・グループ・設定を管理できる管理者ロールです。テナントをまたぐ操作はできません。",
	"system_admin": "システム全体の管理者ロールです。テナントの作成・無効化など、テナントをまたぐ操作を行えます。",
}

var permissionDescriptionCopy = map[string]string{
	"AdminUserRead":        "ユーザーの一覧と詳細を閲覧します。",
	"AdminUserCreate":      "ユーザーを新規作成します。",
	"AdminUserUpdate":      "ユーザーのプロフィール・ロール・有効状態を更新します。",
	"AdminUserDelete":      "ユーザーを削除します。",
	"AdminClientsManage":   "アプリケーションを登録・更新・削除します。",
	"AdminConsentsManage":  "ユーザーがアプリケーションに与えた同意を閲覧・取り消します。",
	"AdminTenantsManage":   "テナントの作成・更新・無効化・有効化を行います。",
	"AdminSettingsRead":    "テナントの設定を閲覧します。",
	"AdminSettingsUpdate":  "テナントの設定を更新します。",
	"AdminAuditEventsRead": "監査ログを閲覧します。",
	"AdminKeysRead":        "署名鍵を閲覧します。",
	"AdminKeysRotate":      "署名鍵を更新（ローテーション）します。",
	"AdminGroupsRead":      "グループの一覧と詳細を閲覧します。",
	"AdminGroupsWrite":     "グループの作成・更新・削除とメンバー管理を行います。",
	"AdminAgentsManage":    "AI エージェント (非人間 identity) の登録・更新・無効化・kill・削除と資格情報束縛を行います。",
}

func roleDescription(name, raw string) string {
	if text, ok := roleDescriptionCopy[name]; ok {
		return text
	}
	return sanitizeAdminCopy(raw)
}

func permissionDescription(name, raw string) string {
	if text, ok := permissionDescriptionCopy[name]; ok {
		return text
	}
	return sanitizeAdminCopy(raw)
}

type adminRolePolicyResponse struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Aliases     []string                      `json:"aliases"`
	Permissions []adminRolePermissionResponse `json:"permissions"`
}

type adminRolePermissionResponse struct {
	Name        string                       `json:"name"`
	Action      string                       `json:"action"`
	Description string                       `json:"description"`
	Interfaces  []adminRoleInterfaceResponse `json:"interfaces"`
}

type adminRoleInterfaceResponse struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

func (d Deps) handleListAdminRolePolicies(c *echo.Context) error {
	actor, err := d.resolveAdminActor(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if !slices.Contains(actor.Roles, "admin") && !slices.Contains(actor.Roles, "system_admin") {
		return d.writeAdminAccessError(c, errAdminAccessDenied)
	}
	roles, err := usecases.ListRolePolicies(
		d.SCL,
		actor.Roles,
		requestTenantID(c) == spec.DefaultTenantID && actor.TenantID == spec.DefaultTenantID,
	)
	if err != nil {
		return err
	}
	response := make([]adminRolePolicyResponse, len(roles))
	for i, role := range roles {
		response[i] = toAdminRolePolicyResponse(role)
	}
	return noStoreJSON(c, http.StatusOK, map[string]any{"roles": response})
}

func toAdminRolePolicyResponse(role usecases.RolePolicy) adminRolePolicyResponse {
	permissions := make([]adminRolePermissionResponse, len(role.Permissions))
	for i, permission := range role.Permissions {
		interfaces := make([]adminRoleInterfaceResponse, len(permission.Interfaces))
		for j, iface := range permission.Interfaces {
			interfaces[j] = adminRoleInterfaceResponse(iface)
		}
		permissions[i] = adminRolePermissionResponse{
			Name: permission.Name, Action: permission.Action,
			Description: permissionDescription(permission.Name, permission.Description), Interfaces: interfaces,
		}
	}
	return adminRolePolicyResponse{
		Name: role.Name, Description: roleDescription(role.Name, role.Description), Aliases: slices.Clone(role.Aliases),
		Permissions: permissions,
	}
}
