// Command server is the CronCompose control plane: a Fiber REST API for the UI and
// an mTLS gRPC endpoint for agents.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/api"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/config"
	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
	"github.com/croncompose/croncompose/control-plane/internal/db"
	"github.com/croncompose/croncompose/control-plane/internal/logger"
	"github.com/croncompose/croncompose/control-plane/internal/notify"
	"github.com/croncompose/croncompose/control-plane/internal/pki"
	"github.com/croncompose/croncompose/control-plane/internal/retention"
	"github.com/croncompose/croncompose/control-plane/internal/secrets"
	"github.com/croncompose/croncompose/control-plane/internal/setup"
	"github.com/croncompose/croncompose/control-plane/internal/updates"
)

func main() {
	seedAndExit := flag.Bool("seed-and-exit", false, "upsert SEED_ADMIN_* from env/.env into the database and exit")
	flag.Parse()
	if err := run(*seedAndExit); err != nil {
		os.Stderr.WriteString("fatal: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run(seedAndExit bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.LogLevel)
	log.Info("starting control plane", "env", cfg.Env)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// db.Open doesn't ping, so check now purely to log a clear signal at boot. A
	// failed ping here is not fatal: the server still starts, /healthz reports
	// "degraded" until Postgres is reachable, and the UI shows a setup banner.
	pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
	dbErr := pool.Ping(pingCtx)
	pingCancel()
	if dbErr != nil {
		if cfg.AutoBootstrapDB {
			log.Info("postgres not reachable; attempting auto-bootstrap")
			bootCtx, bootCancel := context.WithTimeout(ctx, 3*time.Minute)
			res, bootErr := setup.Bootstrap(bootCtx, pool, setup.Options{
				DatabaseURL:   cfg.DatabaseURL,
				ProjectRoot:   cfg.ProjectRoot,
				MigrationsDir: cfg.MigrationsDir,
				Mode:          setup.ParseMode(cfg.BootstrapMode),
			})
			bootCancel()
			if bootErr != nil {
				log.Warn("auto-bootstrap failed; control plane is starting in a degraded state", "err", bootErr)
			} else {
				log.Info("auto-bootstrap complete",
					"started_postgres", res.StartedPostgres,
					"migrations", res.Migrations)
				rePingCtx, rePingCancel := context.WithTimeout(ctx, 3*time.Second)
				dbErr = pool.Ping(rePingCtx)
				rePingCancel()
			}
		}
		if dbErr != nil {
			log.Warn("postgres not reachable; control plane is starting in a degraded state", "err", dbErr)
		}
	}
	if dbErr == nil {
		log.Info("postgres reachable")
	}

	seedErr := auth.SeedAdmin(ctx, log, auth.NewStore(pool), cfg.SeedAdminEmail, cfg.SeedAdminPassword)
	if seedAndExit {
		if dbErr != nil {
			return dbErr
		}
		return seedErr
	}

	oidcCfg := auth.OIDCConfig{
		IssuerURL:    cfg.OIDCIssuerURL,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		DefaultRole:  cfg.OIDCDefaultRole,
	}
	oidc, err := auth.NewOIDC(ctx, oidcCfg, log)
	if err != nil {
		return err
	}

	box, err := cryptobox.New(cfg.SecretsMasterKey)
	if err != nil {
		return err
	}

	bundle, err := pki.LoadOrCreate(cfg.TLSDir, cfg.TLSHosts)
	if err != nil {
		return err
	}
	log.Info("pki ready", "dir", cfg.TLSDir, "hosts", cfg.TLSHosts)

	secretStore := secrets.NewStore(pool, box)
	notifier := notify.NewNotifier(notify.NewStore(pool), pool, log, publicUIBase(cfg))
	gw := agentgw.New(cfg.GRPCAddr, log, pool, bundle, secretStore)
	gw.SetFailedRunHook(notifier)
	gw.SetRunLogMaxBytes(cfg.RunLogMaxBytes)
	manualUpdate := agentgw.ParseUpdatePolicy(
		cfg.AgentUpdateVersion, cfg.AgentUpdateURL, cfg.AgentUpdateSHA256, cfg.AgentUpdateRestart)
	gw.SetUpdatePolicy(manualUpdate)
	if err := gw.Start(ctx); err != nil {
		return err
	}
	defer gw.Stop()

	pruner := retention.New(pool, log, retention.Config{
		RunDays:       cfg.RetentionRunDays,
		RunLogDays:    cfg.RetentionRunLogDays,
		AuditDays:     cfg.RetentionAuditDays,
		OperationDays: cfg.RetentionOperationDays,
	})
	if pruner.Enabled() {
		go pruner.Start(ctx)
	} else {
		log.Info("retention pruning is off; set RETENTION_RUN_LOG_DAYS and RETENTION_RUN_DAYS to enable it")
	}

	var updateCatalog *updates.Catalog
	if cfg.GitHubReleaseRepo != "" && !manualUpdate.Active() {
		updateCatalog = updates.NewCatalog(log, updates.Config{
			Repo:     cfg.GitHubReleaseRepo,
			Restart:  cfg.AgentUpdateRestart,
			Interval: time.Duration(cfg.AgentUpdatePollMinutes) * time.Minute,
		})
		go updateCatalog.Run(ctx)
		log.Info("watching github for agent releases", "repo", cfg.GitHubReleaseRepo)
	}

	app := api.New(api.Deps{
		Log:                log,
		Pool:               pool,
		Gateway:            gw,
		PKI:                bundle,
		GRPCAddr:           cfg.GRPCAddr,
		SessionSecret:      []byte(cfg.SessionSecret),
		PublicHTTPURL:      cfg.PublicHTTPURL,
		PublicGRPCAddr:     cfg.PublicGRPCAddr,
		InstallScriptURL:   cfg.InstallScriptURL,
		WebUpstream:        cfg.WebUpstream,
		Crypto:             box,
		OIDC:               oidc,
		OIDCPostPath:       "/",
		Notifier:           notifier,
		MetricsToken:       cfg.MetricsToken,
		Env:                cfg.Env,
		DatabaseURL:        cfg.DatabaseURL,
		ProjectRoot:        cfg.ProjectRoot,
		MigrationsDir:      cfg.MigrationsDir,
		BootstrapMode:      setup.ParseMode(cfg.BootstrapMode),
		SeedAdminEmail:     cfg.SeedAdminEmail,
		SeedAdminPassword:  cfg.SeedAdminPassword,
		Updates:            updateCatalog,
		ManualUpdatePolicy: manualUpdate,
	})

	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(cfg.HTTPAddr) }()
	log.Info("http listening", "addr", cfg.HTTPAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Error("http stopped", "err", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = app.ShutdownWithContext(shutdownCtx)
	return nil
}

// publicUIBase is the origin notifications link back to. PublicHTTPURL points at the
// REST API (it ends in /api/v1), so deep links have to be built from the base URL, not
// from it. An unset base yields an empty string and links are simply omitted.
func publicUIBase(cfg config.Config) string {
	if cfg.PublicBaseURL != "" {
		return strings.TrimSuffix(cfg.PublicBaseURL, "/")
	}
	return strings.TrimSuffix(strings.TrimSuffix(cfg.PublicHTTPURL, "/"), "/api/v1")
}
