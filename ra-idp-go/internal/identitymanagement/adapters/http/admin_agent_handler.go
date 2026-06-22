package http

import (
	"errors"
	"net/http"
	"slices"
	"time"

	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type agentRegisterRequest struct {
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Kind        *spec.AgentKind `json:"kind"`
	OwnerSub    *string         `json:"owner_sub"`
	Roles       []string        `json:"roles"`
}

type agentUpdateRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Kind        *spec.AgentKind `json:"kind"`
	OwnerSub    *string         `json:"owner_sub"`
	Roles       *[]string       `json:"roles"`
}

type agentCredentialBindRequest struct {
	ClientID string `json:"client_id"`
}

type agentSummaryResponse struct {
	ID          string           `json:"id"`
	TenantID    string           `json:"tenant_id"`
	Name        string           `json:"name"`
	Description *string          `json:"description,omitempty"`
	Kind        spec.AgentKind   `json:"kind"`
	OwnerSub    string           `json:"owner_sub"`
	Status      spec.AgentStatus `json:"status"`
	Roles       []string         `json:"roles"`
	ClientIDs   []string         `json:"client_ids"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   *time.Time       `json:"updated_at,omitempty"`
	DisabledAt  *time.Time       `json:"disabled_at,omitempty"`
	KilledAt    *time.Time       `json:"killed_at,omitempty"`
}

func (d Deps) handleListAgents(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	views, err := idmusecases.ListAgents(c.Request().Context(), d.adminAgentDeps())
	if err != nil {
		return err
	}
	agents := make([]agentSummaryResponse, len(views))
	for i, view := range views {
		agents[i] = toAgentSummaryResponse(view.Agent, view.ClientIDs)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"agents": agents})
}

func (d Deps) handleGetAgent(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	view, err := idmusecases.GetAgent(c.Request().Context(), d.adminAgentDeps(), c.Param("agent_id"))
	if err != nil {
		return d.writeAdminAgentError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAgentSummaryResponse(view.Agent, view.ClientIDs))
}

func (d Deps) handleRegisterAgent(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input agentRegisterRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	ownerSub := ""
	if input.OwnerSub != nil {
		ownerSub = *input.OwnerSub
	}
	agent, err := idmusecases.RegisterAgent(c.Request().Context(), d.adminAgentDeps(), idmusecases.RegisterAgentInput{
		ActorSub: actor.Sub, Name: input.Name, Description: input.Description,
		Kind: input.Kind, OwnerSub: ownerSub, Roles: input.Roles, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeAdminAgentError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusCreated, toAgentSummaryResponse(agent, []string{}))
}

func (d Deps) handleUpdateAgent(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input agentUpdateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	agentID := c.Param("agent_id")
	if _, err := idmusecases.UpdateAgent(c.Request().Context(), d.adminAgentDeps(), idmusecases.UpdateAgentInput{
		ActorSub: actor.Sub, ID: agentID,
		Name: input.Name, Description: input.Description, Kind: input.Kind,
		OwnerSub: input.OwnerSub, Roles: input.Roles, Now: time.Now().UTC(),
	}); err != nil {
		return d.writeAdminAgentError(c, err)
	}
	view, err := idmusecases.GetAgent(c.Request().Context(), d.adminAgentDeps(), agentID)
	if err != nil {
		return d.writeAdminAgentError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAgentSummaryResponse(view.Agent, view.ClientIDs))
}

func (d Deps) handleDisableAgent(c *echo.Context) error {
	return d.changeAgentStatus(c, func(actorSub, id string) error {
		_, err := idmusecases.SetAgentDisabled(c.Request().Context(), d.adminAgentDeps(), actorSub, id, true, time.Now().UTC())
		return err
	})
}

func (d Deps) handleEnableAgent(c *echo.Context) error {
	return d.changeAgentStatus(c, func(actorSub, id string) error {
		_, err := idmusecases.SetAgentDisabled(c.Request().Context(), d.adminAgentDeps(), actorSub, id, false, time.Now().UTC())
		return err
	})
}

func (d Deps) handleKillAgent(c *echo.Context) error {
	return d.changeAgentStatus(c, func(actorSub, id string) error {
		_, err := idmusecases.KillAgent(c.Request().Context(), d.adminAgentDeps(), actorSub, id, time.Now().UTC())
		return err
	})
}

func (d Deps) handleDeleteAgent(c *echo.Context) error {
	return d.changeAgentStatus(c, func(actorSub, id string) error {
		return idmusecases.DeleteAgent(c.Request().Context(), d.adminAgentDeps(), actorSub, id, time.Now().UTC())
	})
}

func (d Deps) handleBindAgentCredential(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input agentCredentialBindRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if err := idmusecases.BindCredential(c.Request().Context(), d.adminAgentDeps(), actor.Sub, c.Param("agent_id"), input.ClientID, time.Now().UTC()); err != nil {
		return d.writeAdminAgentError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleUnbindAgentCredential(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := idmusecases.UnbindCredential(c.Request().Context(), d.adminAgentDeps(), actor.Sub, c.Param("agent_id"), c.Param("client_id"), time.Now().UTC()); err != nil {
		return d.writeAdminAgentError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

// changeAgentStatus は disable / enable / kill / delete の共通処理 (verify + admin gate
// + 204)。
func (d Deps) changeAgentStatus(c *echo.Context, action func(actorSub, id string) error) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := action(actor.Sub, c.Param("agent_id")); err != nil {
		return d.writeAdminAgentError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) adminAgentDeps() idmusecases.AdminAgentDeps {
	return idmusecases.AdminAgentDeps{AgentRepo: d.AgentRepo, ClientRepo: d.ClientRepo, UserRepo: d.UserRepo, Emit: d.Emit}
}

func toAgentSummaryResponse(agent *spec.Agent, clientIDs []string) agentSummaryResponse {
	if clientIDs == nil {
		clientIDs = []string{}
	}
	return agentSummaryResponse{
		ID: agent.ID, TenantID: agent.TenantID, Name: agent.Name, Description: agent.Description,
		Kind: agent.Kind, OwnerSub: agent.OwnerSub, Status: agent.Status,
		Roles: slices.Clone(agent.Roles), ClientIDs: slices.Clone(clientIDs),
		CreatedAt: agent.CreatedAt, UpdatedAt: agent.UpdatedAt,
		DisabledAt: agent.DisabledAt, KilledAt: agent.KilledAt,
	}
}

func (d Deps) writeAdminAgentError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, idmusecases.ErrAgentNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "agent_not_found", "エージェントが存在しません")
	case errors.Is(err, idmusecases.ErrAgentClientNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "client_not_found", "クライアントが存在しません")
	case errors.Is(err, idmusecases.ErrAgentNameConflict):
		return core.WriteBrowserError(c, http.StatusConflict, "agent_name_conflict", "エージェント名は既に使用されています")
	case errors.Is(err, idmusecases.ErrAgentNameEmpty):
		return core.WriteBrowserError(c, http.StatusBadRequest, "agent_name_required", "エージェント名は必須です")
	case errors.Is(err, idmusecases.ErrAgentOwnerRequired):
		return core.WriteBrowserError(c, http.StatusBadRequest, "agent_owner_required", "所有者は必須です")
	case errors.Is(err, idmusecases.ErrAgentOwnerNotFound):
		return core.WriteBrowserError(c, http.StatusBadRequest, "agent_owner_not_found", "所有者ユーザーが存在しません")
	case errors.Is(err, idmusecases.ErrAgentKilled):
		return core.WriteBrowserError(c, http.StatusConflict, "agent_killed", "緊急停止済みのエージェントは変更できません")
	case errors.Is(err, idmusecases.ErrAgentClientBound):
		return core.WriteBrowserError(c, http.StatusConflict, "agent_client_already_bound", "クライアントは別のエージェントに束縛済みです")
	case errors.Is(err, idmusecases.ErrInvalidRole):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_role", "roleが不正です")
	default:
		return err
	}
}
