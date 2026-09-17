package agentgw

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/dbmigrate"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Production change that would fail this test: onHello updating version/os/arch
// but leaving agent_euid_root / agent_service_user stale after reconnect.
func TestOnHelloPersistsEuidAndServiceUser(t *testing.T) {
	dsn := os.Getenv("INTEGRATION_DB_URL")
	if dsn == "" {
		t.Skip("set INTEGRATION_DB_URL to a writable postgres DSN to run this")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := dbmigrate.Apply(ctx, pool, findMigrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	serverID := ids.New()
	if _, err := pool.Exec(ctx, `
		insert into servers (id, name, labels, status, created_at)
		values ($1, 'hello-euid', '{}', 'pending', now())
	`, serverID); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from servers where id = $1`, serverID)
	})

	s := &service{log: slog.Default(), pool: pool}
	if err := s.onHello(ctx, serverID, &agentv1.Hello{
		AgentVersion: "9.9.9",
		Os:           "linux",
		Arch:         "arm64",
		EuidRoot:     true,
		ServiceUser:  "croncompose",
	}); err != nil {
		t.Fatal(err)
	}

	var euidRoot bool
	var serviceUser, version, status string
	if err := pool.QueryRow(ctx, `
		select agent_euid_root, coalesce(agent_service_user,''), agent_version, status
		from servers where id = $1
	`, serverID).Scan(&euidRoot, &serviceUser, &version, &status); err != nil {
		t.Fatal(err)
	}
	if !euidRoot || serviceUser != "croncompose" || version != "9.9.9" || status != "online" {
		t.Fatalf("euid_root=%v service_user=%q version=%q status=%q", euidRoot, serviceUser, version, status)
	}

	if err := s.onHello(ctx, serverID, &agentv1.Hello{
		AgentVersion: "9.9.9",
		Os:           "linux",
		Arch:         "arm64",
		EuidRoot:     false,
		ServiceUser:  "",
	}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		select agent_euid_root, coalesce(agent_service_user,'') from servers where id = $1
	`, serverID).Scan(&euidRoot, &serviceUser); err != nil {
		t.Fatal(err)
	}
	if euidRoot {
		t.Fatal("euid_root should update to false")
	}
	if serviceUser != "croncompose" {
		t.Fatalf("empty Hello.service_user must not clear last known user, got %q", serviceUser)
	}
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(wd, "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		wd = filepath.Dir(wd)
	}
	t.Fatal("could not find migrations/ dir")
	return ""
}
