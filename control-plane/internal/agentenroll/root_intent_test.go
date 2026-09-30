package agentenroll

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/dbmigrate"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

func boolPtr(b bool) *bool { return &b }

// Production change that would fail this test: treating a missing run_as_root as false,
// which would switch root access off every time an older agent re-enrolls.
func TestRootIntent(t *testing.T) {
	cases := []struct {
		name        string
		in          *bool
		wantEnabled bool
		wantOK      bool
	}{
		{name: "old agent, field absent", in: nil, wantOK: false},
		{name: "installed as root", in: boolPtr(true), wantEnabled: true, wantOK: true},
		{name: "installed as service user", in: boolPtr(false), wantEnabled: false, wantOK: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enabled, ok := rootIntent(tc.in)
			if enabled != tc.wantEnabled || ok != tc.wantOK {
				t.Fatalf("rootIntent=(%v,%v) want (%v,%v)", enabled, ok, tc.wantEnabled, tc.wantOK)
			}
		})
	}
}

// Production change that would fail this test: a json tag other than run_as_root, or a
// plain bool, which cannot tell "false" from "absent".
func TestRequestDecodesRunAsRoot(t *testing.T) {
	cases := map[string]*bool{
		`{"token":"t"}`:                     nil,
		`{"token":"t","run_as_root":true}`:  boolPtr(true),
		`{"token":"t","run_as_root":false}`: boolPtr(false),
	}
	for body, want := range cases {
		var req Request
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		switch {
		case want == nil && req.RunAsRoot != nil:
			t.Errorf("%s: RunAsRoot=%v want nil", body, *req.RunAsRoot)
		case want != nil && (req.RunAsRoot == nil || *req.RunAsRoot != *want):
			t.Errorf("%s: RunAsRoot=%v want %v", body, req.RunAsRoot, *want)
		}
	}
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir not found at %s: %v", dir, err)
	}
	return dir
}

// Production change that would fail this test: a root install leaving agent_root_enabled
// false, or a re-enroll without the field rewriting the flag.
func TestRecordRootIntentUpdatesFlag(t *testing.T) {
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
	if _, err := dbmigrate.Apply(ctx, pool, migrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	serverID := ids.New()
	if _, err := pool.Exec(ctx, `
		insert into servers (id, name, labels, status, created_at)
		values ($1, 'enroll-root-intent', '{}', 'pending', now())
	`, serverID); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from servers where id = $1`, serverID)
	})

	h := &handler{log: slog.Default(), pool: pool}
	read := func() (enabled bool, changed bool) {
		if err := pool.QueryRow(ctx, `
			select agent_root_enabled, agent_root_changed_at is not null from servers where id = $1
		`, serverID).Scan(&enabled, &changed); err != nil {
			t.Fatal(err)
		}
		return enabled, changed
	}

	if err := h.recordRootIntent(ctx, serverID, boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if enabled, changed := read(); !enabled || !changed {
		t.Fatalf("root install: enabled=%v changed_at_set=%v, want true/true", enabled, changed)
	}

	if err := h.recordRootIntent(ctx, serverID, nil); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := read(); !enabled {
		t.Fatal("an agent without run_as_root must leave the flag alone")
	}

	if err := h.recordRootIntent(ctx, serverID, boolPtr(false)); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := read(); enabled {
		t.Fatal("re-enrolling as a service user must clear the flag")
	}
}
