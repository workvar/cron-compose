package servers

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// handler holds dependencies the HTTP handlers need.
type handler struct {
	log       *slog.Logger
	store     *Store
	enroll    *EnrollmentStore
	tokenTTL  time.Duration
	audit     audit.Writer
	endpoints Endpoints
	gateway   *agentgw.Gateway
	passkeys  passkeyChecker
	stepUp    stepUpVerifier
	roots     agentRootStore
	rootCmd   agentRootSender
}

func (h *handler) list(c fiber.Ctx) error {
	rows, err := h.store.List(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

func (h *handler) create(c fiber.Ctx) error {
	var in CreateInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.Name == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_name", errors.New("name is required"))
	}
	srv := Server{
		ID:          ids.New(),
		Name:        in.Name,
		Description: in.Description,
		Labels:      coalesceLabels(in.Labels),
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
	}
	if err := h.store.Insert(c.Context(), srv); err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "insert_failed", err)
	}

	token, expiresAt, err := h.enroll.Issue(c.Context(), srv.ID, h.tokenTTL)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "enroll_token_failed", err)
	}

	resp := CreateResponse{
		Server: srv,
		Enrollment: EnrollmentTokenResponse{
			Token:     token,
			ExpiresAt: expiresAt,
		},
		InstallCommand:     h.installCommand(token, false),
		InstallCommandRoot: h.installCommand(token, true),
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "server.create", "server", srv.ID, map[string]any{"name": srv.Name})
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// installCommand builds the one-line installer invocation for a freshly issued
// enrollment token. asRoot sets AGENT_RUN_AS_ROOT=1 so the agent runs as root from
// the start instead of the croncompose service user.
func (h *handler) installCommand(token string, asRoot bool) string {
	vars := fmt.Sprintf("TOKEN=%s AGENT_ENROLL_HTTP=%s AGENT_GRPC_ADDR=%s",
		token, h.endpoints.PublicHTTPURL, h.endpoints.PublicGRPCAddr)
	if asRoot {
		vars += " AGENT_RUN_AS_ROOT=1"
	}
	return fmt.Sprintf("curl -sSL %s | sudo %s bash", h.endpoints.InstallScriptURL, vars)
}

func (h *handler) get(c fiber.Ctx) error {
	srv, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	if h.gateway != nil {
		if msg := h.gateway.UpdateProgress().AgentRootError(srv.ID); msg != "" {
			srv.AgentRootError = msg
		}
	}
	return c.JSON(srv)
}

func (h *handler) patch(c fiber.Ctx) error {
	var in PatchInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	srv, err := h.store.Patch(c.Context(), c.Params("id"), in)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "patch_failed", err)
	}
	// Labels can change which label-targeted jobs apply; re-sync the server.
	if h.gateway != nil {
		if err := h.gateway.PushFullSync(c.Context(), srv.ID); err != nil && !errors.Is(err, agentgw.ErrAgentOffline) {
			h.log.Warn("post-patch sync failed", "server_id", srv.ID, "err", err)
		}
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "server.update", "server", srv.ID, nil)
	return c.JSON(srv)
}

func (h *handler) delete(c fiber.Ctx) error {
	id := c.Params("id")
	err := h.store.Delete(c.Context(), id)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "server.delete", "server", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handler) issueToken(c fiber.Ctx) error {
	srv, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	token, expiresAt, err := h.enroll.Issue(c.Context(), srv.ID, h.tokenTTL)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "enroll_token_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "server.enrollment_token.issue", "server", srv.ID, nil)
	return c.JSON(IssueTokenResponse{
		Enrollment:         EnrollmentTokenResponse{Token: token, ExpiresAt: expiresAt},
		InstallCommand:     h.installCommand(token, false),
		InstallCommandRoot: h.installCommand(token, true),
	})
}

func coalesceLabels(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return in
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": err.Error(),
		},
	})
}
