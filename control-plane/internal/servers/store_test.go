package servers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/dbmigrate"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

func TestSetAgentRootEnabledPersistsFlag(t *testing.T) {
	env := newServerTestEnv(t)
	id := ids.New()
	if err := env.store.Insert(env.ctx, Server{
		ID: id, Name: "pi", Labels: map[string]string{}, Status: "pending", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := env.store.SetAgentRootEnabled(env.ctx, id, true, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AgentRootEnabled {
		t.Fatal("expected agent_root_enabled")
	}
	if got.AgentRootChangedBy == nil || *got.AgentRootChangedBy != env.userID {
		t.Fatalf("changed_by=%v", got.AgentRootChangedBy)
	}
	if got.AgentRootChangedAt == nil {
		t.Fatal("expected changed_at")
	}
	if got.AgentEuidRoot {
		t.Fatal("euid_root must stay false until the agent reports it")
	}
}

func TestSetAgentRootEnabledNotFound(t *testing.T) {
	env := newServerTestEnv(t)
	_, err := env.store.SetAgentRootEnabled(env.ctx, "missing", true, env.userID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

type serverTestEnv struct {
	ctx    context.Context
	store  *Store
	userID string
}

func newServerTestEnv(t *testing.T) *serverTestEnv {
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

	migrationsDir := findMigrationsDir(t)
	if _, err := dbmigrate.Apply(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	userID := ids.New()
	_, err = pool.Exec(ctx, `
		insert into users (id, email, name, role, created_at)
		values ($1, $2, 'Server Test', 'admin', now())
	`, userID, userID+"@server-test.local")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from servers where agent_root_changed_by = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from users where id = $1`, userID)
	})

	return &serverTestEnv{ctx: ctx, store: NewStore(pool), userID: userID}
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
