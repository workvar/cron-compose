package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWebAuthnStoreInsertAndList(t *testing.T) {
	s := newTestWebAuthnStore(t)
	c := Cred{ID: "c1", UserID: "u1", CredentialID: []byte{1, 2, 3}, PublicKey: []byte{9}, Name: "Laptop"}
	if err := s.InsertCredential(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListByUser(context.Background(), "u1")
	if err != nil || len(list) != 1 || list[0].Name != "Laptop" {
		t.Fatalf("got %+v err=%v", list, err)
	}
}

func newTestWebAuthnStore(t *testing.T) *WebAuthnStore {
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

	applyWebAuthnMigrations(t, ctx, pool)

	_, err = pool.Exec(ctx, `
		insert into users (id, email, name, role, created_at)
		values ('u1', 'webauthn-test@local', 'WebAuthn Test', 'admin', now())
		on conflict (id) do nothing
	`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return NewWebAuthnStore(pool)
}

func applyWebAuthnMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := findWebAuthnMigrationsDir(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", e.Name(), err)
		}
	}
}

func findWebAuthnMigrationsDir(t *testing.T) string {
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
