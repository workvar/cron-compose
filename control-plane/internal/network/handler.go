package network

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Register mounts network endpoints under /servers/:id/network.
func Register(r fiber.Router, log *slog.Logger, gw *agentgw.Gateway) {
	h := &handler{log: log, gw: gw}
	r.Get("/servers/:id/network", auth.RequireRole("viewer"), h.status)
	r.Get("/servers/:id/network/wifi/scan", auth.RequireRole("viewer"), h.wifiScan)
	r.Post("/servers/:id/network/wifi/save", auth.RequireRole("admin"), h.wifiSave)
	r.Post("/servers/:id/network/wifi/psk", auth.RequireRole("admin"), h.wifiPSK)
	r.Post("/servers/:id/network/wifi/connect", auth.RequireRole("admin"), h.wifiConnect)
	r.Post("/servers/:id/network/wifi/disconnect", auth.RequireRole("admin"), h.wifiDisconnect)
	r.Post("/servers/:id/network/wifi/forget", auth.RequireRole("admin"), h.wifiForget)
	r.Post("/servers/:id/network/wired", auth.RequireRole("admin"), h.wiredSet)
	r.Get("/servers/:id/network/bluetooth", auth.RequireRole("viewer"), h.bluetooth)
	r.Post("/servers/:id/network/bluetooth/scan", auth.RequireRole("admin"), h.btScan)
	r.Post("/servers/:id/network/bluetooth/pair", auth.RequireRole("admin"), h.btPairStream)
	r.Post("/servers/:id/network/bluetooth/connect", auth.RequireRole("admin"), h.btConnect)
	r.Post("/servers/:id/network/bluetooth/disconnect", auth.RequireRole("admin"), h.btDisconnect)
	r.Post("/servers/:id/network/bluetooth/forget", auth.RequireRole("admin"), h.btForget)
	r.Post("/servers/:id/network/bluetooth/pan", auth.RequireRole("admin"), h.btPAN)
	r.Post("/servers/:id/network/bluetooth/pin", auth.RequireRole("admin"), h.btPin)
	r.Get("/servers/:id/network/cellular", auth.RequireRole("viewer"), h.cellular)
	r.Post("/servers/:id/network/cellular/connect", auth.RequireRole("admin"), h.cellConnect)
	r.Post("/servers/:id/network/cellular/disconnect", auth.RequireRole("admin"), h.cellDisconnect)
	r.Post("/servers/:id/network/cellular/apn", auth.RequireRole("admin"), h.cellAPN)
}

type handler struct {
	log *slog.Logger
	gw  *agentgw.Gateway
}

func (h *handler) status(c fiber.Ctx) error {
	return h.unary(c, "status", "{}")
}

func (h *handler) wifiScan(c fiber.Ctx) error {
	return h.unary(c, "wifi_scan", "{}")
}

func (h *handler) wifiSave(c fiber.Ctx) error {
	return h.unaryBody(c, "wifi_save")
}

func (h *handler) wifiPSK(c fiber.Ctx) error {
	return h.unaryBody(c, "wifi_update_psk")
}

func (h *handler) wifiConnect(c fiber.Ctx) error {
	return h.unaryBody(c, "wifi_connect")
}

func (h *handler) wifiDisconnect(c fiber.Ctx) error {
	return h.unaryBody(c, "wifi_disconnect")
}

func (h *handler) wifiForget(c fiber.Ctx) error {
	return h.unaryBody(c, "wifi_forget")
}

func (h *handler) wiredSet(c fiber.Ctx) error {
	return h.unaryBody(c, "wired_set")
}

func (h *handler) bluetooth(c fiber.Ctx) error {
	return h.unary(c, "bt_devices", "{}")
}

func (h *handler) btScan(c fiber.Ctx) error {
	return h.unary(c, "bt_scan", "{}")
}

func (h *handler) btConnect(c fiber.Ctx) error {
	return h.unaryBody(c, "bt_connect")
}

func (h *handler) btDisconnect(c fiber.Ctx) error {
	return h.unaryBody(c, "bt_disconnect")
}

func (h *handler) btForget(c fiber.Ctx) error {
	return h.unaryBody(c, "bt_forget")
}

func (h *handler) btPAN(c fiber.Ctx) error {
	var in map[string]any
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	action, _ := in["action"].(string)
	op := "bt_pan_connect"
	if action == "disconnect" {
		op = "bt_pan_disconnect"
	}
	b, _ := json.Marshal(in)
	return h.unary(c, op, string(b))
}

func (h *handler) btPin(c fiber.Ctx) error {
	serverID := c.Params("id")
	var in struct {
		RequestID string `json:"request_id"`
		PIN       string `json:"pin"`
	}
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	if in.RequestID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "request_id is required"})
	}
	res, err := h.gw.SendNetworkPinReply(c.Context(), serverID, in.RequestID, in.PIN)
	if err != nil {
		return mapErr(c, err)
	}
	return resultJSON(c, res)
}

func (h *handler) cellular(c fiber.Ctx) error {
	return h.unary(c, "cell_modems", "{}")
}

func (h *handler) cellConnect(c fiber.Ctx) error {
	return h.unaryBody(c, "cell_connect")
}

func (h *handler) cellDisconnect(c fiber.Ctx) error {
	return h.unaryBody(c, "cell_disconnect")
}

func (h *handler) cellAPN(c fiber.Ctx) error {
	return h.unaryBody(c, "cell_set_apn")
}

func (h *handler) unaryBody(c fiber.Ctx, op string) error {
	raw := c.Body()
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if !json.Valid(raw) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	return h.unary(c, op, string(raw))
}

func (h *handler) unary(c fiber.Ctx, op, argsJSON string) error {
	serverID := c.Params("id")
	res, err := h.gw.SendNetworkRequest(c.Context(), serverID, op, argsJSON)
	if err != nil {
		return mapErr(c, err)
	}
	return resultJSON(c, res)
}

// btPairStream runs bluetooth pairing with SSE so the UI can answer PIN challenges.
func (h *handler) btPairStream(c fiber.Ctx) error {
	serverID := c.Params("id")
	raw := c.Body()
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if !json.Valid(raw) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}

	requestID, sub, err := h.gw.BeginNetworkRequest(serverID, "bt_pair", string(raw))
	if err != nil {
		return mapErr(c, err)
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.gw.CloseNetworkRequest(requestID)
		writeSSE(w, "connected", fmt.Sprintf(`{"request_id":%q}`, requestID))

		timer := time.NewTimer(3 * time.Minute)
		defer timer.Stop()
		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case ev := <-sub.Events():
				if ev == nil {
					continue
				}
				payload, _ := json.Marshal(fiber.Map{
					"request_id": ev.GetRequestId(),
					"kind":       ev.GetKind(),
					"chunk":      string(ev.GetData()),
					"percent":    ev.GetPercent(),
					"seq":        ev.GetSeq(),
					"pin_device": ev.GetPinDevice(),
					"pin_method": ev.GetPinMethod(),
					"pin_prompt": ev.GetPinPrompt(),
				})
				writeSSE(w, ev.GetKind(), string(payload))
			case res := <-sub.Result():
				if res == nil {
					writeSSE(w, "done", `{"status":"failed","error":"empty result"}`)
					return
				}
				body := fiber.Map{
					"request_id":  res.GetRequestId(),
					"status":      res.GetStatus(),
					"error":       res.GetError(),
					"result_json": res.GetResultJson(),
				}
				payload, _ := json.Marshal(body)
				writeSSE(w, "done", string(payload))
				return
			case <-keepalive.C:
				_, _ = w.WriteString(": keepalive\n\n")
				_ = w.Flush()
			case <-timer.C:
				writeSSE(w, "done", `{"status":"failed","error":"agent timed out"}`)
				return
			}
		}
	})
	return nil
}

func resultJSON(c fiber.Ctx, res *agentv1.NetworkResult) error {
	if res.GetError() != "" && res.GetStatus() == "failed" {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":       res.GetError(),
			"status":      res.GetStatus(),
			"result_json": res.GetResultJson(),
		})
	}
	var parsed any
	if s := res.GetResultJson(); s != "" {
		_ = json.Unmarshal([]byte(s), &parsed)
	}
	return c.JSON(fiber.Map{
		"request_id": res.GetRequestId(),
		"status":     res.GetStatus(),
		"error":      res.GetError(),
		"result":     parsed,
	})
}

func writeSSE(w *bufio.Writer, event, data string) {
	_, _ = w.WriteString("event: " + event + "\ndata: " + data + "\n\n")
	_ = w.Flush()
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
