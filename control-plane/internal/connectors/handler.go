package connectors

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// handler holds the dependencies the connector endpoints need. The gateway and audit
// writer are nil for nobody: the read endpoints ignore them, the write endpoints
// require them.
type handler struct {
	log     *slog.Logger
	store   *Store
	gateway *agentgw.Gateway
	audit   audit.Writer
}

// audited records one connector mutation against the acting user.
func (h *handler) audited(c fiber.Ctx, action, targetID string, meta map[string]any) {
	h.audit.Write(c.Context(), auth.CurrentUserID(c), action, "connector", targetID, meta)
}

// listAll: GET /connectors. Every connector across all servers (overview).
func (h *handler) listAll(c fiber.Ctx) error {
	rows, err := h.store.ListAll(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

// listByServer: GET /servers/:id/connectors.
func (h *handler) listByServer(c fiber.Ctx) error {
	rows, err := h.store.ListByServer(c.Context(), c.Params("id"))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

// get: GET /connectors/:id.
func (h *handler) get(c fiber.Ctx) error {
	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(conn)
}

// listResources: GET /connectors/:id/resources.
func (h *handler) listResources(c fiber.Ctx) error {
	rows, err := h.store.ListResources(c.Context(), c.Params("id"))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": err.Error(),
		},
	})
}
