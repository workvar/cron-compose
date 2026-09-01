package setup

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Handler exposes dev-only database bootstrap endpoints.
type Handler struct {
	log            *slog.Logger
	env            string
	databaseURL    string
	projectRoot    string
	migrations     string
	bootstrapMode  Mode
	seedAdminEmail string
	seedAdminPass  string
	pool           *pgxpool.Pool
}

// NewHandler builds a setup handler. Bootstrap routes are disabled outside dev.
func NewHandler(
	log *slog.Logger,
	env, databaseURL, projectRoot, migrationsDir string,
	bootstrapMode Mode,
	seedEmail, seedPass string,
	pool *pgxpool.Pool,
) *Handler {
	return &Handler{
		log:            log,
		env:            env,
		databaseURL:    databaseURL,
		projectRoot:    projectRoot,
		migrations:     migrationsDir,
		bootstrapMode:  bootstrapMode,
		seedAdminEmail: seedEmail,
		seedAdminPass:  seedPass,
		pool:           pool,
	}
}

// Register mounts open /setup routes under /api/v1.
func Register(v1 fiber.Router, h *Handler) {
	v1.Get("/setup/status", h.status)
	v1.Post("/setup/bootstrap", h.bootstrap)
}

func (h *Handler) status(c fiber.Ctx) error {
	if !h.devEnabled() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	db := "ok"
	if err := h.ping(c); err != nil {
		db = "down"
	}
	return c.JSON(fiber.Map{
		"db":             db,
		"can_bootstrap":  CanBootstrap(h.bootstrapMode),
		"bootstrap_mode": string(h.bootstrapMode),
		"auto_bootstrap": true,
	})
}

func (h *Handler) bootstrap(c fiber.Ctx) error {
	if !h.devEnabled() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := Bootstrap(ctx, h.pool, Options{
		DatabaseURL:   h.databaseURL,
		ProjectRoot:   h.projectRoot,
		MigrationsDir: h.migrations,
		Mode:          h.bootstrapMode,
	})
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "error",
			"error":  err.Error(),
		})
	}
	if err := h.ping(c); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "error",
			"error":  "database still unreachable after bootstrap",
		})
	}
	auth.SeedAdmin(c.Context(), h.log, auth.NewStore(h.pool), h.seedAdminEmail, h.seedAdminPass)
	return c.JSON(fiber.Map{
		"status":           "ok",
		"started_postgres": res.StartedPostgres,
		"migrations":       res.Migrations,
	})
}

func (h *Handler) devEnabled() bool {
	return h.env == "dev"
}

func (h *Handler) ping(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	return h.pool.Ping(ctx)
}
