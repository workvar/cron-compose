// Package setup bootstraps a dev database: ensure local Postgres is reachable (or start
// the docker-compose service when explicitly configured), provision role/db, migrate.
package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/dbmigrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Mode selects how bootstrap brings Postgres up when it is unreachable.
type Mode string

const (
	ModeLocal  Mode = "local"  // default: local install, no Docker
	ModeDocker Mode = "docker" // docker compose postgres service
	ModeNone   Mode = "none"   // disabled
)

// Options configures a bootstrap run.
type Options struct {
	DatabaseURL   string
	ProjectRoot   string // repo root; empty means discover from cwd
	MigrationsDir string // empty means <ProjectRoot>/migrations
	Mode          Mode   // empty means local
}

// Result describes what bootstrap did.
type Result struct {
	StartedPostgres bool
	Migrations      int
}

// Bootstrap ensures Postgres is reachable and migrations are applied.
func Bootstrap(ctx context.Context, pool *pgxpool.Pool, opts Options) (Result, error) {
	var res Result
	if opts.DatabaseURL == "" {
		return res, errors.New("database URL is required")
	}
	mode := opts.Mode
	if mode == "" {
		mode = ModeLocal
	}
	if mode == ModeNone {
		return res, errors.New("database bootstrap is disabled")
	}

	root, err := resolveProjectRoot(opts.ProjectRoot)
	if err != nil {
		return res, err
	}
	migrationsDir := opts.MigrationsDir
	if migrationsDir == "" {
		migrationsDir = filepath.Join(root, "migrations")
	}

	if err := ping(ctx, pool); err != nil {
		switch mode {
		case ModeDocker:
			if !DockerAvailable() {
				return res, fmt.Errorf("postgres is not reachable and docker is not available: %w", err)
			}
			if err := startPostgresDocker(ctx, root); err != nil {
				return res, fmt.Errorf("start postgres: %w", err)
			}
			res.StartedPostgres = true
			if err := waitForDB(ctx, opts.DatabaseURL); err != nil {
				return res, fmt.Errorf("wait for postgres: %w", err)
			}
		case ModeLocal:
			dsn, parseErr := parseDSN(opts.DatabaseURL)
			if parseErr != nil {
				return res, parseErr
			}
			started, ensureErr := ensureLocalPostgres(ctx, dsn)
			if ensureErr != nil {
				return res, ensureErr
			}
			res.StartedPostgres = started
			if err := waitForDB(ctx, opts.DatabaseURL); err != nil {
				return res, fmt.Errorf("wait for postgres: %w", err)
			}
		default:
			return res, fmt.Errorf("postgres is not reachable: %w", err)
		}
	}

	n, err := dbmigrate.Apply(ctx, pool, migrationsDir)
	if err != nil {
		return res, fmt.Errorf("apply migrations: %w", err)
	}
	res.Migrations = n
	return res, nil
}

// CanBootstrap reports whether automatic setup is possible for the given mode.
func CanBootstrap(mode Mode) bool {
	switch mode {
	case ModeNone:
		return false
	case ModeDocker:
		return DockerAvailable()
	default:
		return true
	}
}

// DockerAvailable reports whether docker compose can be invoked.
func DockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	if exec.Command("docker", "compose", "version").Run() == nil {
		return true
	}
	_, err := exec.LookPath("docker-compose")
	return err == nil
}

func ping(ctx context.Context, pool *pgxpool.Pool) error {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return pool.Ping(pingCtx)
}

func waitForDB(ctx context.Context, url string) error {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		if err := pingURL(ctx, url); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("postgres did not become ready: %w", lastErr)
}

func startPostgresDocker(ctx context.Context, root string) error {
	cmd, err := dockerComposeCmd(root)
	if err != nil {
		return err
	}
	cmd = append(cmd, "up", "-d", "postgres")
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", cmd, err, out)
	}
	return nil
}

func dockerComposeCmd(root string) ([]string, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, errors.New("docker is not installed")
	}
	composeFile := filepath.Join(root, "docker-compose.yml")
	if exec.Command("docker", "compose", "version").Run() == nil {
		return []string{"docker", "compose", "-f", composeFile}, nil
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return []string{"docker-compose", "-f", composeFile}, nil
	}
	return nil, errors.New("docker compose is not available")
}

func resolveProjectRoot(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if v := os.Getenv("CRONCOMPOSE_ROOT"); v != "" {
		return v, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 12; i++ {
		if dirExists(filepath.Join(dir, "migrations")) || fileExists(filepath.Join(dir, "docker-compose.yml")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("could not find project root (set CRONCOMPOSE_ROOT or run from the repo)")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
