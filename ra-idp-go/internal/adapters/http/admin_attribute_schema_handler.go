package http

import (
	"errors"
	"net/http"
	"time"

	"ra-idp-go/internal/spec"
	tenantusecases "ra-idp-go/internal/tenancy/usecases"

	"github.com/labstack/echo/v5"
)

type attributeSchemaResponse struct {
	TenantID   string              `json:"tenant_id"`
	Attributes []spec.AttributeDef `json:"attributes"`
	Builtin    []spec.AttributeDef `json:"builtin"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

type attributeSchemaUpdateRequest struct {
	Attributes []spec.AttributeDef `json:"attributes"`
}

func toAttributeSchemaResponse(schema *spec.TenantAttributeSchema) attributeSchemaResponse {
	attributes := schema.Attributes
	if attributes == nil {
		attributes = []spec.AttributeDef{}
	}
	return attributeSchemaResponse{
		TenantID:   schema.TenantID,
		Attributes: attributes,
		Builtin:    spec.BuiltinAttributeDefs(),
		UpdatedAt:  schema.UpdatedAt,
	}
}

func (d Deps) handleGetAttributeSchema(c *echo.Context) error {
	actor, err := d.requireTenantAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	schema, err := tenantusecases.GetAttributeSchema(c.Request().Context(), d.AttrSchemaRepo, actor.TenantID)
	if err != nil {
		return err
	}
	return noStoreJSON(c, http.StatusOK, toAttributeSchemaResponse(schema))
}

func (d Deps) handleUpdateAttributeSchema(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireTenantAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	var input attributeSchemaUpdateRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	schema, err := tenantusecases.UpdateAttributeSchema(
		c.Request().Context(), d.AttrSchemaRepo, actor.TenantID, input.Attributes, time.Now().UTC(),
	)
	if err != nil {
		if errors.Is(err, tenantusecases.ErrInvalidAttributeSchema) {
			return writeBrowserError(c, http.StatusBadRequest, "invalid_attribute_schema", "属性定義が不正です")
		}
		return err
	}
	if d.Emit != nil {
		keys := make([]string, len(schema.Attributes))
		for i, def := range schema.Attributes {
			keys[i] = def.Key
		}
		d.Emit(&spec.TenantAttributeSchemaUpdated{
			At: time.Now().UTC(), ActorSub: actor.Sub, TenantID: actor.TenantID, AttributeKeys: keys,
		})
	}
	return noStoreJSON(c, http.StatusOK, toAttributeSchemaResponse(schema))
}
