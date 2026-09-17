package auth

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

func TestWebAuthnStoreInsertAndList(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	credID := ids.New()
	c := Cred{
		ID:           credID,
		UserID:       env.userID,
		CredentialID: []byte{1, 2, 3, byte(time.Now().UnixNano() & 0xff)},
		PublicKey:    []byte{9},
		Name:         "Laptop",
	}
	if err := env.store.InsertCredential(env.ctx, c); err != nil {
		t.Fatal(err)
	}
	list, err := env.store.ListByUser(env.ctx, env.userID)
	if err != nil || len(list) != 1 || list[0].Name != "Laptop" {
		t.Fatalf("got %+v err=%v", list, err)
	}
}

func TestWebAuthnStoreGetByCredentialID(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	credBytes := []byte{4, 5, 6, byte(time.Now().UnixNano() & 0xff)}
	c := Cred{
		ID:           ids.New(),
		UserID:       env.userID,
		CredentialID: credBytes,
		PublicKey:    []byte{9},
		Name:         "Phone",
	}
	if err := env.store.InsertCredential(env.ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := env.store.GetByCredentialID(env.ctx, credBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Phone" || got.UserID != env.userID {
		t.Fatalf("got %+v", got)
	}
}

func TestWebAuthnStoreDelete(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	credPK := ids.New()
	c := Cred{
		ID:           credPK,
		UserID:       env.userID,
		CredentialID: []byte{7, 8, 9, byte(time.Now().UnixNano() & 0xff)},
		PublicKey:    []byte{9},
		Name:         "YubiKey",
	}
	if err := env.store.InsertCredential(env.ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := env.store.Delete(env.ctx, env.userID, credPK); err != nil {
		t.Fatal(err)
	}
	list, err := env.store.ListByUser(env.ctx, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
	if err := env.store.Delete(env.ctx, env.userID, credPK); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: got %v want ErrNotFound", err)
	}
	if err := env.store.Delete(env.ctx, ids.New(), credPK); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong user delete: got %v want ErrNotFound", err)
	}
}

func TestWebAuthnStoreUpdateSignCount(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	credPK := ids.New()
	c := Cred{
		ID:           credPK,
		UserID:       env.userID,
		CredentialID: []byte{10, 11, 12, byte(time.Now().UnixNano() & 0xff)},
		PublicKey:    []byte{9},
		Name:         "Token",
	}
	if err := env.store.InsertCredential(env.ctx, c); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := env.store.UpdateSignCount(env.ctx, credPK, 42); err != nil {
		t.Fatal(err)
	}
	got, err := env.store.GetByCredentialID(env.ctx, c.CredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SignCount != 42 {
		t.Fatalf("sign_count: got %d want 42", got.SignCount)
	}
	if got.LastUsedAt == nil {
		t.Fatal("last_used_at not set")
	}
	if got.LastUsedAt.Before(before.Add(-time.Second)) {
		t.Fatalf("last_used_at too old: %v", got.LastUsedAt)
	}
}

func TestWebAuthnStoreChallengePutTake(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	chID := ids.New()
	chBytes := []byte{0xab, 0xcd}
	expires := time.Now().Add(2 * time.Minute)
	ch := Challenge{
		ID:        chID,
		UserID:    &env.userID,
		Purpose:   "enroll",
		Challenge: chBytes,
		ExpiresAt: expires,
	}
	if err := env.store.PutChallenge(env.ctx, ch); err != nil {
		t.Fatal(err)
	}
	got, err := env.store.TakeChallenge(env.ctx, chID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Purpose != "enroll" || string(got.Challenge) != string(chBytes) {
		t.Fatalf("got %+v", got)
	}
	if got.UserID == nil || *got.UserID != env.userID {
		t.Fatalf("user_id: got %+v", got.UserID)
	}
}

func TestWebAuthnStoreChallengeTakeTwice(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	chID := ids.New()
	ch := Challenge{
		ID:        chID,
		UserID:    &env.userID,
		Purpose:   "login",
		Challenge: []byte{1, 2},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := env.store.PutChallenge(env.ctx, ch); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.TakeChallenge(env.ctx, chID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.TakeChallenge(env.ctx, chID); !errors.Is(err, ErrChallengeNotFound) {
		t.Fatalf("second take: got %v want ErrChallengeNotFound", err)
	}
}

type webAuthnTestEnv struct {
	t      *testing.T
	ctx    context.Context
	pool   *pgxpool.Pool
	store  *WebAuthnStore
	userID string
}

func newWebAuthnTestEnv(t *testing.T) *webAuthnTestEnv {
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

	migrationsDir := findWebAuthnMigrationsDir(t)
	if _, err := dbmigrate.Apply(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	userID := ids.New()
	_, err = pool.Exec(ctx, `
		insert into users (id, email, name, role, created_at)
		values ($1, $2, 'WebAuthn Test', 'admin', now())
	`, userID, userID+"@webauthn-test.local")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from webauthn_credentials where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from webauthn_challenges where user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `delete from users where id = $1`, userID)
	})

	return &webAuthnTestEnv{
		t:      t,
		ctx:    ctx,
		pool:   pool,
		store:  NewWebAuthnStore(pool),
		userID: userID,
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
