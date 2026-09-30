package agentgw

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/dbmigrate"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// seedRootServer opens the integration database and inserts a server whose stored
// agent_root_enabled flag is rootEnabled.
func seedRootServer(t *testing.T, rootEnabled bool) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
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
		insert into servers (id, name, labels, status, agent_root_enabled, created_at)
		values ($1, 'hello-root-repush', '{}', 'pending', $2, now())
	`, serverID, rootEnabled); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from servers where id = $1`, serverID)
	})
	return ctx, pool, serverID
}

// The reported bug end to end: the flag says root, the agent reconnects as non-root,
// and onHello must push the command once instead of leaving the toggle stuck.
// Production change that would fail this test: onHello dropping the repushRootCommand
// call, or re-sending on every Hello.
func TestOnHelloRepushesRootCommandWhenFlagAndAgentDisagree(t *testing.T) {
	ctx, pool, serverID := seedRootServer(t, true)
	s := &service{
		log:      slog.Default(),
		pool:     pool,
		registry: NewRegistry(),
		progress: NewUpdateProgressTracker(),
		rootSent: newRootDelivery(),
	}
	conn := s.registry.Add(serverID)
	notRoot := &agentv1.Hello{AgentVersion: "9.9.9", Os: "linux", Arch: "amd64", EuidRoot: false}

	if err := s.onHello(ctx, serverID, notRoot); err != nil {
		t.Fatal(err)
	}
	if cmd := drain(t, conn); cmd == nil || !cmd.GetEnabled() {
		t.Fatalf("first Hello must re-send the elevate command, got %v", cmd)
	}

	if err := s.onHello(ctx, serverID, notRoot); err != nil {
		t.Fatal(err)
	}
	if drain(t, conn) != nil {
		t.Fatal("a repeated disagreement must not re-send the command")
	}
}

func TestOnHelloDoesNotRepushWhenAgentAlreadyMatchesFlag(t *testing.T) {
	ctx, pool, serverID := seedRootServer(t, true)
	s := &service{
		log:      slog.Default(),
		pool:     pool,
		registry: NewRegistry(),
		progress: NewUpdateProgressTracker(),
		rootSent: newRootDelivery(),
	}
	conn := s.registry.Add(serverID)

	if err := s.onHello(ctx, serverID, &agentv1.Hello{EuidRoot: true}); err != nil {
		t.Fatal(err)
	}
	if drain(t, conn) != nil {
		t.Fatal("agent already root and flag on: nothing to send")
	}
}

// Legacy root agent: flag false, changed_by null, agent runs as root. No demote.
// Production change that would fail this test: dropping the operatorSet gate.
func TestOnHelloLeavesLegacyRootAgentAlone(t *testing.T) {
	ctx, pool, serverID := seedRootServer(t, false)
	s := &service{log: slog.Default(), pool: pool, registry: NewRegistry(), progress: NewUpdateProgressTracker(), rootSent: newRootDelivery()}
	conn := s.registry.Add(serverID)

	if err := s.onHello(ctx, serverID, &agentv1.Hello{EuidRoot: true}); err != nil {
		t.Fatal(err)
	}
	if drain(t, conn) != nil {
		t.Fatal("legacy root agent must not be demoted")
	}
}

// An operator switched root off while the agent was offline: demote once.
func TestOnHelloRepushesDemoteWhenOperatorDisabledRoot(t *testing.T) {
	ctx, pool, serverID := seedRootServer(t, false)
	userID := ids.New()
	if _, err := pool.Exec(ctx, `insert into users (id, email, name, password_hash) values ($1, $2, 'op', 'x')`, userID, userID+"@example.test"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `update servers set agent_root_changed_by = null where id = $1`, serverID)
		_, _ = pool.Exec(context.Background(), `delete from users where id = $1`, userID)
	})
	if _, err := pool.Exec(ctx, `update servers set agent_root_changed_by = $2 where id = $1`, serverID, userID); err != nil {
		t.Fatalf("mark operator: %v", err)
	}
	s := &service{log: slog.Default(), pool: pool, registry: NewRegistry(), progress: NewUpdateProgressTracker(), rootSent: newRootDelivery()}
	conn := s.registry.Add(serverID)

	if err := s.onHello(ctx, serverID, &agentv1.Hello{EuidRoot: true}); err != nil {
		t.Fatal(err)
	}
	if cmd := drain(t, conn); cmd == nil || cmd.GetEnabled() {
		t.Fatalf("operator disable must send one demote, got %v", cmd)
	}
}
