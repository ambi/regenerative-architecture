// /api/account/data_export — エンドユーザー自身の個人データを JSON で取得する
// (self-service, GDPR 第15条 right of access, wi-21)。本ステージは同期生成で、
// 現状 API から得られる profile と接続済みアプリ (consents) をまとめる。
package http

import (
	"net/http"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthusecases "ra-idp-go/internal/oauth2/usecases"

	"github.com/labstack/echo/v5"
)

type accountDataExport struct {
	ExportedAt time.Time                `json:"exported_at"`
	Profile    accountProfileResponse   `json:"profile"`
	Consents   []accountConsentResponse `json:"consents"`
}

func (d Deps) handleExportAccountData(c *echo.Context) error {
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	user, defs, err := authusecases.GetUserProfile(c.Request().Context(), d.accountProfileDeps(), sub)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	consents, err := oauthusecases.ListConsentsForSub(c.Request().Context(), d.adminConsentDeps(), sub)
	if err != nil {
		return err
	}
	consentResponses := make([]accountConsentResponse, len(consents))
	for i, consent := range consents {
		consentResponses[i] = toAccountConsentResponse(consent)
	}
	return noStoreJSON(c, http.StatusOK, accountDataExport{
		ExportedAt: time.Now().UTC(),
		Profile:    toAccountProfileResponse(user, defs),
		Consents:   consentResponses,
	})
}
