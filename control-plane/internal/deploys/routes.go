package deploys

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// RegisterPublic attaches webhook + token-triggered run endpoints (no session required).
func RegisterPublic(r fiber.Router, h *handler, optAuth fiber.Handler) {
	r.Post("/deploys/webhooks/github", h.githubWebhook)
	r.Post("/deploys/webhooks/gitlab", h.gitlabWebhook)
	r.Post("/deploys/:id/runs", optAuth, h.createRun)
}

// Register attaches authenticated deploy and git-connection routes.
func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool, gw *agentgw.Gateway, writer audit.Writer, conns *auth.ConnStore, publicBase, gitlabBase string) *handler {
	h := &handler{
		log: log, store: NewStore(pool), conns: conns, git: NewGitAPI(gitlabBase),
		gateway: gw, audit: writer, public: publicBase,
	}
	r.Get("/git/connections", h.listConnections)
	r.Delete("/git/connections/:provider", h.deleteConnection)
	r.Get("/git/repos", h.listRepos)
	r.Get("/git/inspect", h.inspect)

	r.Get("/deploy-settings", h.getSettings)
	r.Put("/deploy-settings", auth.RequireRole("admin"), h.putSettings)

	r.Get("/deploys", h.list)
	r.Get("/deploys/:id", h.get)
	r.Get("/deploys/:id/workflow", h.workflow)
	r.Get("/deploys/:id/runs", h.listRuns)
	r.Post("/deploys", auth.RequireRole("operator"), h.create)
	r.Patch("/deploys/:id", auth.RequireRole("operator"), h.patch)
	r.Delete("/deploys/:id", auth.RequireRole("operator"), h.remove)

	r.Get("/deploy-runs/:runId", h.getRun)
	r.Get("/deploy-runs/:runId/logs/stream", h.stream)
	r.Post("/deploy-runs/:runId/stdin", auth.RequireRole("operator"), h.stdin)
	return h
}
