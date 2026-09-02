package connectors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

type upsertPortLabelRequest struct {
	ServerID string `json:"server_id"`
	Proto    string `json:"proto"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Label    string `json:"label"`
}

// listLabels: GET /port-labels. Any authenticated role.
func (h *handler) listLabels(c fiber.Ctx) error {
	rows, err := h.store.ListAllLabels(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

// upsertLabel: PUT /port-labels. Operator and above. Empty label deletes the row.
func (h *handler) upsertLabel(c fiber.Ctx) error {
	var req upsertPortLabelRequest
	if err := c.Bind().Body(&req); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	req.ServerID = strings.TrimSpace(req.ServerID)
	req.Address = strings.TrimSpace(req.Address)
	if req.ServerID == "" || req.Address == "" {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", errors.New("server_id and address are required"))
	}
	if req.Port < 1 || req.Port > 65535 {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", fmt.Errorf("port must be 1–65535"))
	}

	label, del, err := NormalizeLabel(req.Label)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}

	proto := req.Proto
	if del {
		if err := h.store.DeleteLabel(c.Context(), req.ServerID, proto, req.Address, req.Port); err != nil {
			return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
		}
		h.audited(c, "port.label.delete", req.ServerID, map[string]any{
			"address": req.Address, "port": req.Port, "proto": proto,
		})
		return c.JSON(fiber.Map{"deleted": true})
	}

	row, err := h.store.UpsertLabel(c.Context(), req.ServerID, proto, req.Address, req.Port, label)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "save_failed", err)
	}
	h.audited(c, "port.label.upsert", req.ServerID, map[string]any{
		"address": req.Address, "port": req.Port, "proto": proto, "label": label,
		"actor": auth.CurrentUserID(c),
	})
	return c.JSON(row)
}
