// Rich Authorization Requests (RFC 9396) の受付・検証ヘルパー (ADR-050)。
//
// /authorize・/par・/token から共通で呼ばれ、authorization_details を JSON から
// パースし、テナント登録済み type に対し fail-closed に検証する。ダウンスコープ
// 判定 (DetailsSubsetOf) 用の type 定義ロードもここに集約する。
package usecases

import (
	"context"
	"encoding/json"
	"strings"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

const errInvalidAuthorizationDetails = "invalid_authorization_details"

// ParseAuthorizationDetails は RFC 9396 authorization_details パラメータ (JSON 配列文字列)
// をパースする。空文字は nil。構文不正・配列でないものは invalid_authorization_details。
func ParseAuthorizationDetails(raw string) ([]spec.AuthorizationDetail, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var details []spec.AuthorizationDetail
	if err := json.Unmarshal([]byte(raw), &details); err != nil {
		return nil, NewOAuthError(errInvalidAuthorizationDetails, "authorization_details の JSON が不正です")
	}
	if len(details) == 0 {
		return nil, NewOAuthError(errInvalidAuthorizationDetails, "authorization_details が空です")
	}
	return details, nil
}

// ValidateAuthorizationDetails は各 detail を登録済み type に対し検証する (fail-closed)。
// repo が nil・未登録 type・スキーマ不適合はすべて拒否する。
func ValidateAuthorizationDetails(ctx context.Context, repo ports.AuthorizationDetailTypeRepository, details []spec.AuthorizationDetail) error {
	if len(details) == 0 {
		return nil
	}
	if repo == nil {
		return NewOAuthError(errInvalidAuthorizationDetails, "authorization_details は受理されていません")
	}
	tenantID := tenancy.TenantID(ctx)
	for _, d := range details {
		if d.Type == "" {
			return NewOAuthError(errInvalidAuthorizationDetails, "authorization_details の type が必要です")
		}
		t, err := repo.FindByType(ctx, tenantID, d.Type)
		if err != nil {
			return err
		}
		if t == nil {
			return NewOAuthError(errInvalidAuthorizationDetails, "未登録の authorization_details type: "+d.Type)
		}
		if err := domain.ValidateAgainstType(d, *t); err != nil {
			return NewOAuthError(errInvalidAuthorizationDetails, err.Error())
		}
	}
	return nil
}

// LoadAuthorizationDetailTypes は details に現れる type の登録定義を map で返す
// (DetailsSubsetOf のダウンスコープ判定用)。未登録 type は map に現れない。
func LoadAuthorizationDetailTypes(ctx context.Context, repo ports.AuthorizationDetailTypeRepository, details []spec.AuthorizationDetail) (map[string]spec.AuthorizationDetailType, error) {
	out := map[string]spec.AuthorizationDetailType{}
	if repo == nil {
		return out, nil
	}
	tenantID := tenancy.TenantID(ctx)
	for _, d := range details {
		if _, ok := out[d.Type]; ok {
			continue
		}
		t, err := repo.FindByType(ctx, tenantID, d.Type)
		if err != nil {
			return nil, err
		}
		if t != nil {
			out[d.Type] = *t
		}
	}
	return out, nil
}
