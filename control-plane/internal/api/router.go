package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/agentenroll"
	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/connectors"
	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
	"github.com/croncompose/croncompose/control-plane/internal/jobs"
	"github.com/croncompose/croncompose/control-plane/internal/notify"
	"github.com/croncompose/croncompose/control-plane/internal/pki"
	"github.com/croncompose/croncompose/control-plane/internal/runs"
	"github.com/croncompose/croncompose/control-plane/internal/secrets"
	"github.com/croncompose/croncompose/control-plane/internal/servers"
	"github.com/croncompose/croncompose/control-plane/internal/setup"
	"github.com/croncompose/croncompose/control-plane/internal/templates"
	"github.com/croncompose/croncompose/control-plane/internal/terminal"
	"github.com/croncompose/croncompose/control-plane/internal/updates"
)

// Deps groups everything the router needs to wire feature packages.
type Deps struct {
	Log              *slog.Logger
	Pool             *pgxpool.Pool
	Gateway          *agentgw.Gateway
	PKI              *pki.Bundle
	GRPCAddr         string
	SessionSecret    []byte
	PublicHTTPURL    string
	PublicGRPCAddr   string
	InstallScriptURL string
	WebUpstream      string // internal Next.js address to reverse-proxy /app to; empty disables
	Crypto           *cryptobox.Box
	OIDC             *auth.OIDC
	OIDCPostPath     string // where to send the browser after a successful SSO login
	// Notifier delivers run-failure notifications. The notify routes need it for the
	// test-delivery endpoint.
	Notifier *notify.Notifier
	// Updates watches GitHub for agent releases and exposes update status to the UI.
	Updates            *updates.Catalog
	ManualUpdatePolicy agentgw.UpdatePolicy
	// MetricsToken, when set, is required as a bearer token on /metrics.
	MetricsToken string

	Env               string
	DatabaseURL       string
	ProjectRoot       string
	MigrationsDir     string
	BootstrapMode     setup.Mode
	SeedAdminEmail    string
	SeedAdminPassword string
}

// New builds a Fiber app with all middleware and routes attached.
func New(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "croncompose-control-plane"})
	app.Use(requestLogger(d.Log))
	app.Use(requestMetrics())
	app.Use(apiPrefixRewrite())

	app.Get("/healthz", healthHandler(d.Pool))
	app.Get("/metrics", requireMetricsToken(d.MetricsToken), metricsHandler())

	userStore := auth.NewStore(d.Pool)
	writer := audit.NewWriter(d.Pool, d.Log)

	v1 := app.Group("/api/v1")
	auth.Register(v1, d.Log, userStore, d.SessionSecret, d.OIDC != nil)
	postPath := d.OIDCPostPath
	if postPath == "" {
		postPath = "/"
	}
	auth.RegisterOIDC(v1, userStore, d.SessionSecret, d.OIDC, postPath)
	agentenroll.Register(v1, d.Log, d.Pool, d.PKI, d.GRPCAddr)
	setup.Register(v1, setup.NewHandler(
		d.Log, d.Env, d.DatabaseURL, d.ProjectRoot, d.MigrationsDir, d.BootstrapMode,
		d.SeedAdminEmail, d.SeedAdminPassword, d.Pool,
	))

	authed := v1.Group("", auth.RequireAuth(d.SessionSecret, userStore, d.Log))
	auth.RegisterMe(authed, d.Log, userStore, d.SessionSecret, d.OIDC != nil)
	servers.Register(authed, d.Log, d.Pool, writer, servers.Endpoints{
		PublicHTTPURL:    d.PublicHTTPURL,
		PublicGRPCAddr:   d.PublicGRPCAddr,
		InstallScriptURL: d.InstallScriptURL,
	}, d.Gateway)
	jobs.Register(authed, d.Log, d.Pool, d.Gateway, writer)
	connectors.Register(authed, d.Log, d.Pool, d.Gateway, writer)
	templates.Register(authed, d.Log, d.Pool, writer)
	runs.Register(authed, d.Log, d.Pool, d.Gateway.Broker())
	terminal.Register(authed, d.Log, d.Gateway, writer, d.PublicHTTPURL)
	audit.Register(authed, d.Log, d.Pool)
	secrets.Register(authed, d.Log, d.Pool, d.Crypto, writer)
	notify.Register(authed, d.Log, d.Pool, writer, d.Notifier)
	updates.Register(authed, d.Log, d.Pool, d.Updates, d.ManualUpdatePolicy, d.Gateway, writer)

	// Single entry point: serve the UI under /app (and bounce / into it) when an
	// upstream is configured. With no upstream, / is an nginx-style welcome page.
	// Registered last so it never shadows API routes.
	mountWeb(app, d.WebUpstream)

	return app
}

// requireMetricsToken gates /metrics behind a bearer token when one is configured.
//
// The endpoint stays open by default because that is what every Prometheus setup
// expects on a private network, and requiring a token nobody configured would just
// break scraping silently. Setting METRICS_TOKEN is the deliberate act of someone who
// has exposed the port.
func requireMetricsToken(token string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if token == "" {
			return c.Next()
		}
		if c.Get("Authorization") == "Bearer "+token {
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "unauthenticated", "message": "metrics token required"},
		})
	}
}
