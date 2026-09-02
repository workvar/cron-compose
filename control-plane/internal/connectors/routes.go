package connectors

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Register attaches the connector routes. See docs/connectors.md.
//
// The gating follows what each call can do to the box, not what it touches:
//
//   - reading inventory and operation history is open to any authenticated role,
//     because it is the same information the dashboard already shows;
//   - lifecycle actions (start/stop/restart/reload) are operator, matching job runs;
//   - anything involving config bytes is admin. Config files hold upstream addresses,
//     internal ports and sometimes credentials, and a bad edit takes a service down,
//     so reading them is admin too, not just writing them.
func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool, gw *agentgw.Gateway, writer audit.Writer) {
	h := &handler{log: log, store: NewStore(pool), gateway: gw, audit: writer}

	// Read: any authenticated role.
	r.Get("/connectors", h.listAll)
	r.Get("/connectors/:id", h.get)
	r.Get("/connectors/:id/resources", h.listResources)
	r.Get("/connectors/:id/operations", h.listOperations)
	r.Get("/connectors/:id/ports", h.listPorts)
	r.Get("/port-labels", h.listLabels)
	r.Put("/port-labels", auth.RequireRole("operator"), h.upsertLabel)
	r.Get("/servers/:id/connectors", h.listByServer)

	// Lifecycle: operator and above.
	r.Post("/connectors/:id/actions", auth.RequireRole("operator"), h.action)

	// Config: admin and above.
	r.Get("/connectors/:id/config", auth.RequireRole("admin"), h.readConfig)
	r.Post("/connectors/:id/config", auth.RequireRole("admin"), h.writeConfig)
	r.Get("/connectors/:id/snapshots", auth.RequireRole("admin"), h.listSnapshots)
	r.Post("/connectors/:id/snapshots/:snapshotID/restore", auth.RequireRole("admin"), h.rollback)
}
