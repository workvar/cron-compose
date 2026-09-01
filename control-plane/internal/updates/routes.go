package updates

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Register attaches update routes. GitHub polling is optional; a manually configured
// AGENT_UPDATE_* policy always takes precedence.
func Register(
	r fiber.Router,
	log *slog.Logger,
	pool *pgxpool.Pool,
	catalog *Catalog,
	manual agentgw.UpdatePolicy,
	gw *agentgw.Gateway,
	writer audit.Writer,
) {
	h := &handler{
		log:     log,
		pool:    pool,
		catalog: catalog,
		manual:  manual,
		gateway: gw,
		audit:   writer,
	}
	r.Get("/updates", h.status)
	admin := auth.RequireRole("admin")
	r.Post("/servers/:id/update", admin, h.triggerServerUpdate)
}
