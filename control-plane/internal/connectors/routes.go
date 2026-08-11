package connectors

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Register attaches the read-only connector routes (Phase A). Every authenticated role
// can read connector status and inventory; mutating routes (validate/apply/lifecycle)
// land in later phases with admin/operator gating. See docs/connectors.md.
func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool) {
	h := &handler{log: log, store: NewStore(pool)}

	r.Get("/connectors", h.listAll)
	r.Get("/connectors/:id", h.get)
	r.Get("/connectors/:id/resources", h.listResources)
	r.Get("/servers/:id/connectors", h.listByServer)
}
