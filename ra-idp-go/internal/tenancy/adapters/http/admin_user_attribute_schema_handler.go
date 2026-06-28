package http

import (
	"errors"
	"net/http"
	"time"

	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"
	tenantusecases "ra-idp-go/internal/tenancy/usecases"

	"github.com/labstack/echo/v5"
)

type UserAttributeSchemaResponse struct {
	TenantID   string                  `json:"tenant_id"`
	Attributes []spec.UserAttributeDef `json:"attributes"`
	Builtin    []spec.UserAttributeDef `json:"builtin"`
	UpdatedAt  time.Time               `json:"updated_at"`
}

type userAttributeSchemaUpdateRequest struct {
	Attributes []spec.UserAttributeDef `json:"attributes"`
}

func toUserAttributeSchemaResponse(schema *spec.TenantUserAttributeSchema) UserAttributeSchemaResponse {
	attributes := schema.Attributes
	if attributes == nil {
		attributes = []spec.UserAttributeDef{}
	}
	return UserAttributeSchemaResponse{
		TenantID:   schema.TenantID,
		Attributes: attributes,
		Builtin:    spec.BuiltinUserAttributeDefs(),
		UpdatedAt:  schema.UpdatedAt,
	}
}

func (d Deps) handleGetUserAttributeSchema(c *echo.Context) error {
	actor, err := d.requireTenantAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	schema, err := tenantusecases.GetUserAttributeSchema(c.Request().Context(), d.AttrSchemaRepo, actor.TenantID)
	if err != nil {
		return err
	}
	return support.NoStoreJSON(c, http.StatusOK, toUserAttributeSchemaResponse(schema))
}

func (d Deps) handleUpdateUserAttributeSchema(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireTenantAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input userAttributeSchemaUpdateRequest
	if err := support.DecodeJSON(c.Request(), &input); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	schema, err := tenantusecases.UpdateUserAttributeSchema(
		c.Request().Context(), d.AttrSchemaRepo, actor.TenantID, input.Attributes, time.Now().UTC(),
	)
	if err != nil {
		if errors.Is(err, tenantusecases.ErrInvalidUserAttributeSchema) {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_attribute_schema", "属性定義が不正です")
		}
		return err
	}
	if d.Emit != nil {
		keys := make([]string, len(schema.Attributes))
		for i, def := range schema.Attributes {
			keys[i] = def.Key
		}
		d.Emit(&spec.TenantUserAttributeSchemaUpdated{
			At: time.Now().UTC(), ActorSub: actor.Sub, TenantID: actor.TenantID, AttributeKeys: keys,
		})
	}
	return support.NoStoreJSON(c, http.StatusOK, toUserAttributeSchemaResponse(schema))
}
