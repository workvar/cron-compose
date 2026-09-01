package setup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// parsedDSN holds the pieces we need from DATABASE_URL for local provisioning.
type parsedDSN struct {
	host   string
	port   string
	user   string
	pass   string
	dbName string
}

func parseDSN(raw string) (parsedDSN, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return parsedDSN{}, fmt.Errorf("parse database url: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return parsedDSN{}, fmt.Errorf("unsupported database scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	user := u.User.Username()
	if user == "" {
		user = "croncompose"
	}
	pass, _ := u.User.Password()
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		dbName = "croncompose"
	}
	return parsedDSN{host: host, port: port, user: user, pass: pass, dbName: dbName}, nil
}

func (d parsedDSN) appURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.PathEscape(d.user), url.PathEscape(d.pass), d.host, d.port, d.dbName)
}

func (d parsedDSN) superURL(superUser, superPass string) string {
	if superPass != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
			url.PathEscape(superUser), url.PathEscape(superPass), d.host, d.port)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/postgres?sslmode=disable",
		url.PathEscape(superUser), d.host, d.port)
}

func defaultSuperuser() string {
	if v := os.Getenv("DB_SUPERUSER"); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		if u, err := user.Current(); err == nil && u.Username != "" {
			return u.Username
		}
	}
	return "postgres"
}

func superuserPassword() string {
	return os.Getenv("DB_SUPERUSER_PASSWORD")
}

// localPostgresReady reports whether something is accepting TCP connections at host:port.
func localPostgresReady(host, port string) bool {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ensureLocalPostgres makes a local Postgres server reachable and ensures the app role
// and database from DATABASE_URL exist. Returns true when it started a service.
func ensureLocalPostgres(ctx context.Context, dsn parsedDSN) (started bool, err error) {
	if !localPostgresReady(dsn.host, dsn.port) {
		if err := startLocalPostgres(ctx); err != nil {
			return false, fmt.Errorf("local postgres is not running: %w", err)
		}
		started = true
		if err := waitForTCP(ctx, dsn.host, dsn.port); err != nil {
			return started, err
		}
	}

	if err := pingURL(ctx, dsn.appURL()); err == nil {
		return started, nil
	}

	super := defaultSuperuser()
	if err := provisionRoleAndDB(ctx, dsn.superURL(super, superuserPassword()), dsn.user, dsn.pass, dsn.dbName); err != nil {
		return started, fmt.Errorf("prepare database: %w", err)
	}
	return started, nil
}

func startLocalPostgres(ctx context.Context) error {
	switch runtime.GOOS {
	case "darwin":
		return startPostgresBrew(ctx)
	case "linux":
		return startPostgresLinux(ctx)
	default:
		return errors.New("start postgres manually on this platform")
	}
}

func startPostgresBrew(ctx context.Context) error {
	if _, err := exec.LookPath("brew"); err != nil {
		return errors.New("install PostgreSQL with Homebrew (brew install postgresql@16) and start it")
	}
	for _, svc := range []string{"postgresql@16", "postgresql@15", "postgresql@14", "postgresql"} {
		cmd := exec.CommandContext(ctx, "brew", "services", "start", svc)
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else if len(out) > 0 && !strings.Contains(string(out), "Formula") {
			// brew prints noise when the formula name is wrong; keep trying.
		}
	}
	return errors.New("could not start postgres via brew services; run: brew services start postgresql@16")
}

func startPostgresLinux(ctx context.Context) error {
	for _, args := range [][]string{
		{"systemctl", "start", "postgresql"},
		{"service", "postgresql", "start"},
	} {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return errors.New("start postgres with systemctl or service postgresql start")
}

func waitForTCP(ctx context.Context, host, port string) error {
	var lastErr error
	for i := 0; i < 30; i++ {
		if localPostgresReady(host, port) {
			return nil
		}
		lastErr = errors.New("port not open")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("postgres did not start listening on %s:%s: %w", host, port, lastErr)
}

func pingURL(ctx context.Context, connURL string) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(pingCtx, connURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return pool.Ping(pingCtx)
}

func provisionRoleAndDB(ctx context.Context, superURL, role, password, dbName string) error {
	cfg, err := pgx.ParseConfig(superURL)
	if err != nil {
		return err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect as superuser: %w", err)
	}
	defer conn.Close(ctx)

	var exists int
	if err = conn.QueryRow(ctx, "select 1 from pg_roles where rolname = $1", role).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		roleID := pgx.Identifier{role}.Sanitize()
		if _, err = conn.Exec(ctx, fmt.Sprintf(`create role %s login password $1`, roleID), password); err != nil {
			return fmt.Errorf("create role %s: %w", role, err)
		}
	} else if err != nil {
		return err
	} else {
		roleID := pgx.Identifier{role}.Sanitize()
		_, _ = conn.Exec(ctx, fmt.Sprintf(`alter role %s login password $1`, roleID), password)
	}

	err = conn.QueryRow(ctx, "select 1 from pg_database where datname = $1", dbName).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		dbID := pgx.Identifier{dbName}.Sanitize()
		roleID := pgx.Identifier{role}.Sanitize()
		if _, err = conn.Exec(ctx, fmt.Sprintf(`create database %s owner %s`, dbID, roleID)); err != nil {
			return fmt.Errorf("create database %s: %w", dbName, err)
		}
	} else if err != nil {
		return err
	}
	return nil
}
