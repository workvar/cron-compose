package hosttools

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Register mounts host-tools detect/install endpoints under /servers/:id/tools.
func Register(r fiber.Router, log *slog.Logger, gw *agentgw.Gateway) {
	h := &handler{log: log, gw: gw}
	r.Get("/servers/:id/tools", auth.RequireRole("admin"), h.detect)
	r.Post("/servers/:id/tools/install", auth.RequireRole("admin"), h.install)
}

type handler struct {
	log *slog.Logger
	gw  *agentgw.Gateway
}

type installBody struct {
	RunAs string `json:"run_as"`
	Tool  string `json:"tool"`
}

func (h *handler) detect(c fiber.Ctx) error {
	serverID := c.Params("id")
	runAs := strings.TrimSpace(c.Query("run_as"))
	res, err := h.gw.SendHostToolsRequest(c.Context(), serverID, "detect", runAs, "")
	if err != nil {
		return mapErr(c, err)
	}
	if res.GetError() != "" && res.GetStatus() == "failed" {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": res.GetError()})
	}
	return c.JSON(fiber.Map{
		"run_as": runAs,
		"tools":  res.GetTools(),
	})
}

func (h *handler) install(c fiber.Ctx) error {
	serverID := c.Params("id")
	var in installBody
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	tool := strings.ToLower(strings.TrimSpace(in.Tool))
	if tool == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tool is required"})
	}
	res, err := h.gw.SendHostToolsRequest(c.Context(), serverID, "install", strings.TrimSpace(in.RunAs), tool)
	if err != nil {
		return mapErr(c, err)
	}
	status := fiber.StatusOK
	if res.GetStatus() == "failed" || res.GetExitCode() != 0 {
		status = fiber.StatusBadGateway
	}
	return c.Status(status).JSON(fiber.Map{
		"run_as":    in.RunAs,
		"tool":      tool,
		"status":    res.GetStatus(),
		"exit_code": res.GetExitCode(),
		"error":     res.GetError(),
		"log":       res.GetLog(),
		"tools":     res.GetTools(),
	})
}

func mapErr(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, agentgw.ErrAgentOffline):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "agent offline"})
	case errors.Is(err, agentgw.ErrCommandTimeout):
		return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{"error": "agent timed out"})
	default:
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
}
