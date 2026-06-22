package http

// SCL interfaces: ListAdminKeys / GetAdminKey / RotateAdminKey (bounded_context: OAuth2)。
// SCL permissions: AdminKeysRead は admin / system_admin、AdminKeysRotate は
// default tenant の system_admin のみ。Rotate は SigningKeyRotated を emit する。

import (
	"net/http"
	"slices"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type AdminKeyResponse struct {
	Kid       string         `json:"kid"`
	Alg       string         `json:"alg"`
	Active    bool           `json:"active"`
	CreatedAt time.Time      `json:"created_at"`
	PublicJWK map[string]any `json:"public_jwk"`
}

type AdminRotateKeyResponse struct {
	Next     AdminKeyResponse  `json:"next"`
	Previous *AdminKeyResponse `json:"previous,omitempty"`
}

func (d Deps) handleListAdminKeys(c *echo.Context) error {
	if err := d.requireKeyReader(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.KeyStore == nil {
		return core.NoStoreJSON(c, http.StatusOK, map[string]any{"keys": []AdminKeyResponse{}})
	}
	keys, err := d.KeyStore.GetAllKeys(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]AdminKeyResponse, len(keys))
	for i, k := range keys {
		out[i] = toAdminKeyResponse(k)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"keys": out})
}

func (d Deps) handleGetAdminKey(c *echo.Context) error {
	if err := d.requireKeyReader(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.KeyStore == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "key_not_found", "署名鍵が存在しません")
	}
	key, err := d.KeyStore.FindByKID(c.Request().Context(), c.Param("kid"))
	if err != nil {
		return err
	}
	if key == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "key_not_found", "署名鍵が存在しません")
	}
	return core.NoStoreJSON(c, http.StatusOK, toAdminKeyResponse(key))
}

func (d Deps) handleRotateAdminKey(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.requireKeyRotator(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.KeyStore == nil {
		return core.WriteBrowserError(c, http.StatusServiceUnavailable, "key_store_unavailable", "署名鍵ストアが構成されていません")
	}
	prev, _ := d.KeyStore.GetActiveKey(c.Request().Context())
	next, err := usecases.RotateSigningKey(c.Request().Context(), usecases.RotateSigningKeyDeps{
		KeyStore: d.KeyStore,
		Emit:     d.Emit,
	}, time.Now().UTC())
	if err != nil {
		return err
	}
	resp := AdminRotateKeyResponse{Next: toAdminKeyResponse(next)}
	if prev != nil {
		previous := toAdminKeyResponse(prev)
		previous.Active = false
		resp.Previous = &previous
	}
	return core.NoStoreJSON(c, http.StatusOK, resp)
}

// requireKeyReader は AdminKeysRead を満たす actor か検証する。
// admin / system_admin のどちらでも通る。テナント制約は無し (鍵は global)。
func (d Deps) requireKeyReader(c *echo.Context) error {
	actor, err := d.ResolveAdminActor(c)
	if err != nil {
		return err
	}
	if !slices.Contains(actor.Roles, "admin") && !slices.Contains(actor.Roles, "system_admin") {
		return core.ErrAdminAccessDenied
	}
	return nil
}

// requireKeyRotator は AdminKeysRotate を満たす actor を返す。
// system_admin かつ default tenant 経路のみ。Rotate 失敗は IdP 全体のトークン
// 発行を停止させるため最も狭いゲートを掛ける。
func (d Deps) requireKeyRotator(c *echo.Context) (*spec.User, error) {
	actor, err := d.ResolveAdminActor(c)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(actor.Roles, "system_admin") {
		return nil, core.ErrAdminAccessDenied
	}
	if core.RequestTenantID(c) != spec.DefaultTenantID {
		return nil, core.ErrAdminAccessDenied
	}
	if actor.TenantID != spec.DefaultTenantID {
		return nil, core.ErrAdminAccessDenied
	}
	return actor, nil
}

func toAdminKeyResponse(k *ports.SigningKey) AdminKeyResponse {
	return AdminKeyResponse{
		Kid:       k.Kid,
		Alg:       string(k.Alg),
		Active:    k.Active,
		CreatedAt: k.CreatedAt,
		PublicJWK: k.PublicJWK,
	}
}
