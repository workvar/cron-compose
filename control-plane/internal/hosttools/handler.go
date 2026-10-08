package hosttools

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Register mounts host-tools detect/install endpoints under /servers/:id/tools.
func Register(r fiber.Router, log *slog.Logger, gw *agentgw.Gateway) {
	h := &handler{log: log, gw: gw}
	r.Get("/servers/:id/tools", auth.RequireRole("admin"), h.detect)
	r.Post("/servers/:id/tools/install", auth.RequireRole("admin"), h.install)
	r.Post("/servers/:id/tools/uninstall", auth.RequireRole("admin"), h.uninstall)
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
	tool := strings.ToLower(strings.TrimSpace(c.Query("tool")))
	res, err := h.gw.SendHostToolsRequest(c.Context(), serverID, "detect", runAs, tool)
	if err != nil {
		return mapErr(c, err)
	}
	if res.GetError() != "" && res.GetStatus() == "failed" {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": res.GetError()})
	}
	return c.JSON(fiber.Map{
		"run_as": runAs,
		"tool":   tool,
		"tools":  res.GetTools(),
	})
}

func (h *handler) install(c fiber.Ctx) error {
	return h.mutateStream(c, "install")
}

func (h *handler) uninstall(c fiber.Ctx) error {
	return h.mutateStream(c, "uninstall")
}

// mutateStream runs install/uninstall and streams HostToolsEvent chunks as SSE
// until HostToolsResult arrives. The UI uses fetch() + ReadableStream (EventSource
// cannot POST).
func (h *handler) mutateStream(c fiber.Ctx, op string) error {
	serverID := c.Params("id")
	var in installBody
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	tool := strings.ToLower(strings.TrimSpace(in.Tool))
	if tool == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tool is required"})
	}

	requestID, sub, err := h.gw.BeginHostToolsRequest(serverID, op, strings.TrimSpace(in.RunAs), tool)
	if err != nil {
		return mapErr(c, err)
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.gw.CloseHostToolsRequest(requestID)
		_, _ = w.WriteString(": connected\n\n")
		_ = w.Flush()

		timer := time.NewTimer(15 * time.Minute)
		defer timer.Stop()
		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case ev := <-sub.Logs():
				if ev == nil {
					continue
				}
				payload, _ := json.Marshal(fiber.Map{
					"request_id": ev.GetRequestId(),
					"kind":       ev.GetKind(),
					"chunk":      string(ev.GetData()),
					"percent":    ev.GetPercent(),
					"seq":        ev.GetSeq(),
				})
				writeSSE(w, "log", string(payload))
			case res := <-sub.Result():
				// Flush any chunks that arrived with the final result.
			drain:
				for {
					select {
					case ev := <-sub.Logs():
						if ev == nil {
							continue
						}
						payload, _ := json.Marshal(fiber.Map{
							"request_id": ev.GetRequestId(),
							"kind":       ev.GetKind(),
							"chunk":      string(ev.GetData()),
							"percent":    ev.GetPercent(),
							"seq":        ev.GetSeq(),
						})
						writeSSE(w, "log", string(payload))
					default:
						break drain
					}
				}
				if res == nil {
					writeSSE(w, "done", `{"status":"failed","error":"empty result"}`)
					return
				}
				writeDone(w, op, in.RunAs, tool, res)
				return
			case <-keepalive.C:
				_, _ = w.WriteString(": keepalive\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			case <-timer.C:
				writeSSE(w, "done", fmt.Sprintf(
					`{"op":%q,"tool":%q,"status":"failed","error":"agent timed out","exit_code":1}`,
					op, tool,
				))
				return
			}
		}
	})
	return nil
}

func writeSSE(w *bufio.Writer, event, data string) {
	_, _ = w.WriteString("event: " + event + "\ndata: " + data + "\n\n")
	_ = w.Flush()
}

func writeDone(w *bufio.Writer, op, runAs, tool string, res *agentv1.HostToolsResult) {
	body := fiber.Map{
		"op":        op,
		"run_as":    runAs,
		"tool":      tool,
		"status":    res.GetStatus(),
		"exit_code": res.GetExitCode(),
		"error":     res.GetError(),
		"log":       res.GetLog(),
		"tools":     res.GetTools(),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		writeSSE(w, "done", `{"status":"failed","error":"marshal failed"}`)
		return
	}
	writeSSE(w, "done", string(payload))
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
