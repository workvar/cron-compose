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
	"github.com/croncompose/croncompose/control-plane/internal/terminal"
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
}

// New builds a Fiber app with all middleware and routes attached.
func New(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "croncompose-control-plane"})
	app.Use(requestLogger(d.Log))
	app.Use(requestMetrics())
	app.Use(apiPrefixRewrite())

	app.Get("/healthz", healthHandler(d.Pool))
	app.Get("/metrics", metricsHandler())

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

	authed := v1.Group("", auth.RequireAuth(d.SessionSecret, userStore, d.Log))
	servers.Register(authed, d.Log, d.Pool, writer, servers.Endpoints{
		PublicHTTPURL:    d.PublicHTTPURL,
		PublicGRPCAddr:   d.PublicGRPCAddr,
		InstallScriptURL: d.InstallScriptURL,
	}, d.Gateway)
	jobs.Register(authed, d.Log, d.Pool, d.Gateway, writer)
	connectors.Register(authed, d.Log, d.Pool)
	runs.Register(authed, d.Log, d.Pool, d.Gateway.Broker())
	terminal.Register(authed, d.Log, d.Gateway, writer, d.PublicHTTPURL)
	audit.Register(authed, d.Log, d.Pool)
	secrets.Register(authed, d.Log, d.Pool, d.Crypto, writer)
	notify.Register(authed, d.Log, d.Pool, writer)

	// Single entry point: serve the UI under /app (and bounce / into it) when an
	// upstream is configured. Registered last so it never shadows API routes.
	mountWeb(app, d.WebUpstream)

	return app
}
