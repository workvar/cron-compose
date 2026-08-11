package terminal

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Register attaches the web terminal WebSocket endpoint. It is admin/owner only: an
// interactive shell on a managed host is the most powerful action in the product.
// Mounted on the authenticated group, so RequireAuth has already run. publicURL seeds the
// Origin allowlist used to reject cross-site WebSocket handshakes.
func Register(r fiber.Router, log *slog.Logger, gw *agentgw.Gateway, writer audit.Writer, publicURL string) {
	h := newHandler(log, gw, writer, publicURL)
	r.Get("/servers/:id/terminal/ws", auth.RequireRole("admin"), h.ws)
}
