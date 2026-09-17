package servers

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Register attaches the servers routes with role gating.
//
// viewer: list, get
// admin:  create, patch, delete, issue enrollment token, agent-root toggle
// Endpoints describes the externally reachable addresses agents need to hit. They
// land in the install command shown after `POST /servers`.
type Endpoints struct {
	PublicHTTPURL    string
	PublicGRPCAddr   string
	InstallScriptURL string
}

func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool, writer audit.Writer, ep Endpoints, gw *agentgw.Gateway, stepUp auth.StepUp) {
	store := NewStore(pool)
	if stepUp == nil {
		stepUp = auth.DisabledStepUp()
	}
	h := &handler{
		log:       log,
		store:     store,
		enroll:    NewEnrollmentStore(pool),
		tokenTTL:  30 * time.Minute,
		audit:     writer,
		endpoints: ep,
		gateway:   gw,
		passkeys:  stepUp,
		stepUp:    stepUp,
		roots:     store,
	}
	if gw != nil {
		h.rootCmd = gw
	}
	r.Get("/servers", h.list)
	r.Get("/servers/:id", h.get)

	admin := auth.RequireRole("admin")
	r.Post("/servers", admin, h.create)
	r.Patch("/servers/:id", admin, h.patch)
	r.Delete("/servers/:id", admin, h.delete)
	r.Post("/servers/:id/enrollment-token", admin, h.issueToken)
	r.Post("/servers/:id/agent-root", admin, h.setAgentRoot)
}
