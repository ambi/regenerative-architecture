package http

// SCL interfaces: ListAdminKeys / GetAdminKey / RotateAdminKey (component: Trust)。
// SCL permissions: AdminKeysRead は admin / system_admin、AdminKeysRotate は
// default tenant の system_admin のみ。Rotate は SigningKeyRotated を emit する。

import (
	"net/http"
	"slices"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type adminKeyResponse struct {
	Kid       string         `json:"kid"`
	Alg       string         `json:"alg"`
	Active    bool           `json:"active"`
	CreatedAt time.Time      `json:"created_at"`
	PublicJWK map[string]any `json:"public_jwk"`
}

type adminRotateKeyResponse struct {
	Next     adminKeyResponse  `json:"next"`
	Previous *adminKeyResponse `json:"previous,omitempty"`
}

func (d Deps) handleListAdminKeys(c *echo.Context) error {
	if _, err := d.requireKeyReader(c); err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.KeyStore == nil {
		return noStoreJSON(c, http.StatusOK, map[string]any{"keys": []adminKeyResponse{}})
	}
	keys, err := d.KeyStore.GetAllKeys(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]adminKeyResponse, len(keys))
	for i, k := range keys {
		out[i] = toAdminKeyResponse(k)
	}
	return noStoreJSON(c, http.StatusOK, map[string]any{"keys": out})
}

func (d Deps) handleGetAdminKey(c *echo.Context) error {
	if _, err := d.requireKeyReader(c); err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.KeyStore == nil {
		return writeBrowserError(c, http.StatusNotFound, "key_not_found", "署名鍵が存在しません")
	}
	key, err := d.KeyStore.FindByKID(c.Request().Context(), c.Param("kid"))
	if err != nil {
		return err
	}
	if key == nil {
		return writeBrowserError(c, http.StatusNotFound, "key_not_found", "署名鍵が存在しません")
	}
	return noStoreJSON(c, http.StatusOK, toAdminKeyResponse(key))
}

func (d Deps) handleRotateAdminKey(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.requireKeyRotator(c); err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.KeyStore == nil {
		return writeBrowserError(c, http.StatusServiceUnavailable, "key_store_unavailable", "署名鍵ストアが構成されていません")
	}
	prev, _ := d.KeyStore.GetActiveKey(c.Request().Context())
	next, err := usecases.RotateSigningKey(c.Request().Context(), usecases.RotateSigningKeyDeps{
		KeyStore: d.KeyStore,
		Emit:     d.Emit,
	}, time.Now().UTC())
	if err != nil {
		return err
	}
	resp := adminRotateKeyResponse{Next: toAdminKeyResponse(next)}
	if prev != nil {
		previous := toAdminKeyResponse(prev)
		previous.Active = false
		resp.Previous = &previous
	}
	return noStoreJSON(c, http.StatusOK, resp)
}

// requireKeyReader は AdminKeysRead を満たす actor を返す。
// admin / system_admin のどちらでも通る。テナント制約は無し (鍵は global)。
func (d Deps) requireKeyReader(c *echo.Context) (*spec.User, error) {
	actor, err := d.resolveAdminActor(c)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(actor.Roles, "admin") && !slices.Contains(actor.Roles, "system_admin") {
		return nil, errAdminAccessDenied
	}
	return actor, nil
}

// requireKeyRotator は AdminKeysRotate を満たす actor を返す。
// system_admin かつ default tenant 経路のみ。Rotate 失敗は IdP 全体のトークン
// 発行を停止させるため最も狭いゲートを掛ける。
func (d Deps) requireKeyRotator(c *echo.Context) (*spec.User, error) {
	actor, err := d.resolveAdminActor(c)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(actor.Roles, "system_admin") {
		return nil, errAdminAccessDenied
	}
	if requestTenantID(c) != spec.DefaultTenantID {
		return nil, errAdminAccessDenied
	}
	if actor.TenantID != spec.DefaultTenantID {
		return nil, errAdminAccessDenied
	}
	return actor, nil
}

func (d Deps) resolveAdminActor(c *echo.Context) (*spec.User, error) {
	authn, err := d.resolveAuthentication(c)
	if err != nil {
		return nil, err
	}
	if authn == nil || authn.AuthenticationPending {
		return nil, errAdminAuthenticationRequired
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.DisabledAt != nil {
		return nil, errAdminAccessDenied
	}
	return user, nil
}

func toAdminKeyResponse(k *ports.SigningKey) adminKeyResponse {
	return adminKeyResponse{
		Kid:       k.Kid,
		Alg:       string(k.Alg),
		Active:    k.Active,
		CreatedAt: k.CreatedAt,
		PublicJWK: k.PublicJWK,
	}
}
