package updates

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/servers"
)

type handler struct {
	log     *slog.Logger
	pool    *pgxpool.Pool
	catalog *Catalog
	manual  agentgw.UpdatePolicy
	gateway *agentgw.Gateway
	audit   audit.Writer
}

// ServerStatus is one row in GET /updates.
type ServerStatus struct {
	ServerID        string `json:"server_id"`
	ServerName      string `json:"server_name"`
	Status          string `json:"status"`
	CurrentVersion  string `json:"current_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	CanUpdate       bool   `json:"can_update"`
}

// StatusResponse is returned by GET /updates.
type StatusResponse struct {
	LatestVersion string         `json:"latest_version,omitempty"`
	ReleaseURL    string         `json:"release_url,omitempty"`
	PublishedAt   *string        `json:"published_at,omitempty"`
	CheckError    string         `json:"check_error,omitempty"`
	Items         []ServerStatus `json:"items"`
}

func (h *handler) status(c fiber.Ctx) error {
	policy := h.effectivePolicy()
	rel, ok, checkErr := h.catalog.Snapshot()

	resp := StatusResponse{Items: []ServerStatus{}}
	if ok {
		resp.LatestVersion = rel.Version
		resp.ReleaseURL = rel.ReleaseURL
		if !rel.PublishedAt.IsZero() {
			s := rel.PublishedAt.UTC().Format("2006-01-02T15:04:05Z")
			resp.PublishedAt = &s
		}
	}
	if checkErr != "" && !ok && !policy.Active() {
		resp.CheckError = checkErr
	}

	rows, err := servers.NewStore(h.pool).List(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}

	for _, srv := range rows {
		item := ServerStatus{
			ServerID:       srv.ID,
			ServerName:     srv.Name,
			Status:         srv.Status,
			CurrentVersion: srv.AgentVersion,
		}
		if policy.Active() && srv.AgentVersion != "" {
			item.UpdateAvailable = !agentgw.VersionsEqual(srv.AgentVersion, policy.Version) &&
				agentgw.VersionNewer(srv.AgentVersion, policy.Version)
			up := policy.For(srv.AgentVersion, srv.OS, srv.Arch)
			item.CanUpdate = up != nil && srv.Status == "online"
		}
		resp.Items = append(resp.Items, item)
	}
	return c.JSON(resp)
}

func (h *handler) triggerServerUpdate(c fiber.Ctx) error {
	serverID := c.Params("id")
	srv, err := servers.NewStore(h.pool).Get(c.Context(), serverID)
	if errors.Is(err, servers.ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	if srv.Status != "online" {
		return jsonError(c, fiber.StatusConflict, "agent_offline",
			errors.New("agent is not connected; update when it is online"))
	}
	if srv.AgentVersion == "" {
		return jsonError(c, fiber.StatusConflict, "no_version",
			errors.New("agent version is unknown"))
	}

	policy := h.effectivePolicy()
	if !policy.Active() {
		return jsonError(c, fiber.StatusServiceUnavailable, "no_release",
			errors.New("no agent release is available to update to"))
	}
	up := policy.For(srv.AgentVersion, srv.OS, srv.Arch)
	if up == nil {
		if agentgw.VersionsEqual(srv.AgentVersion, policy.Version) {
			return jsonError(c, fiber.StatusConflict, "already_current",
				errors.New("agent is already on the latest version"))
		}
		return jsonError(c, fiber.StatusConflict, "unsupported_platform",
			errors.New("no checksum is available for this server's platform"))
	}

	if err := h.gateway.SendAgentUpdate(serverID, up); err != nil {
		if errors.Is(err, agentgw.ErrAgentOffline) {
			return jsonError(c, fiber.StatusServiceUnavailable, "agent_offline", err)
		}
		return jsonError(c, fiber.StatusInternalServerError, "send_failed", err)
	}

	h.audit.Write(c.Context(), auth.CurrentUserID(c), "server.agent_update", "server", serverID, map[string]any{
		"from": srv.AgentVersion,
		"to":   up.GetTargetVersion(),
	})
	return c.JSON(fiber.Map{
		"status":         "offered",
		"target_version": up.GetTargetVersion(),
	})
}

func (h *handler) effectivePolicy() agentgw.UpdatePolicy {
	if h.manual.Active() {
		return h.manual
	}
	return h.catalog.Policy()
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": err.Error(),
		},
	})
}
