package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

func TestRelyingPartyFromPublicURL(t *testing.T) {
	rp, err := relyingParty("https://cron.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if rp.ID != "cron.example.com" {
		t.Fatalf("id=%q", rp.ID)
	}
}

func TestRelyingPartyOriginFromPublicHTTPURL(t *testing.T) {
	rp, err := relyingParty("https://cron.example.com/api")
	if err != nil {
		t.Fatal(err)
	}
	if rp.ID != "cron.example.com" {
		t.Fatalf("id=%q", rp.ID)
	}
	if len(rp.Origins) != 1 || rp.Origins[0] != "https://cron.example.com" {
		t.Fatalf("origins=%v", rp.Origins)
	}
}

func TestRelyingPartyRejectsEmpty(t *testing.T) {
	if _, err := relyingParty(""); err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestExpiredChallengeIsRejected(t *testing.T) {
	ch := &Challenge{ExpiresAt: time.Now().Add(-time.Minute)}
	if err := rejectExpiredChallenge(ch); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("got %v want ErrChallengeExpired", err)
	}
}

func TestLiveChallengeIsAccepted(t *testing.T) {
	ch := &Challenge{ExpiresAt: time.Now().Add(time.Minute)}
	if err := rejectExpiredChallenge(ch); err != nil {
		t.Fatal(err)
	}
}

func TestTakeLiveChallengeRejectsExpired(t *testing.T) {
	env := newWebAuthnTestEnv(t)
	chID := ids.New()
	ch := Challenge{
		ID:        chID,
		UserID:    &env.userID,
		Purpose:   purposeLogin,
		Challenge: []byte(`{"challenge":"x"}`),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	if err := env.store.PutChallenge(env.ctx, ch); err != nil {
		t.Fatal(err)
	}
	if _, err := takeLiveChallenge(env.ctx, env.store, chID); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("got %v want ErrChallengeExpired", err)
	}
}

func TestConfigPasskeyLoginWhenRPDerived(t *testing.T) {
	app := fiber.New()
	h := newHandler(nil, nil, []byte("test-secret-at-least-16"), false, nil, OAuthProvider{}, OAuthProvider{})
	h.publicURL = "https://cron.example.com"
	app.Get("/auth/config", h.config)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/auth/config", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["passkey_login"] != true {
		t.Fatalf("passkey_login=%v body=%s", got["passkey_login"], body)
	}
}

func TestConfigPasskeyLoginWhenRPMissing(t *testing.T) {
	app := fiber.New()
	h := newHandler(nil, nil, []byte("test-secret-at-least-16"), false, nil, OAuthProvider{}, OAuthProvider{})
	app.Get("/auth/config", h.config)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/auth/config", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["passkey_login"] != false {
		t.Fatalf("passkey_login=%v body=%s", got["passkey_login"], body)
	}
}

func TestNewWebAuthnFromPublicURL(t *testing.T) {
	wa, err := newWebAuthn("https://cron.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if wa.Config.RPID != "cron.example.com" {
		t.Fatalf("rpid=%q", wa.Config.RPID)
	}
	if len(wa.Config.RPOrigins) != 1 || wa.Config.RPOrigins[0] != "https://cron.example.com" {
		t.Fatalf("origins=%v", wa.Config.RPOrigins)
	}
}

func TestPasskeyRoutesAbsentWhenRPMissing(t *testing.T) {
	app := fiber.New()
	v1 := app.Group("/api/v1")
	RegisterPasskeys(v1, v1, nil, nil, nil, nil, "")
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/begin", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d want 404/405", resp.StatusCode)
	}
}

func TestLoginFinishRejectsMissingChallenge(t *testing.T) {
	app := fiber.New()
	v1 := app.Group("/api/v1")
	RegisterPasskeys(v1, v1, nil, nil, nil, []byte("test-secret-at-least-16"), "https://cron.example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish", strings.NewReader(`{"credential":{}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d want 400 body=%s", resp.StatusCode, body)
	}
}

func TestCredToPublicKeyStoresJSONNotRawCOSE(t *testing.T) {
	credID := []byte{0x01, 0x02, 0x03, 0x04}
	cose := []byte{0xa5, 0x01, 0x02, 0x03}
	in := &webauthn.Credential{
		ID:        credID,
		PublicKey: cose,
		Flags:     webauthn.CredentialFlags{BackupEligible: true, UserVerified: true},
		Authenticator: webauthn.Authenticator{
			SignCount: 7,
		},
	}
	blob, err := credToPublicKey(in)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(blob) {
		t.Fatalf("public_key column must be JSON, got %q", blob)
	}
	var raw map[string]any
	if err := json.Unmarshal(blob, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["id"]; !ok {
		t.Fatalf("JSON missing credential id: %s", blob)
	}
	flags, _ := raw["flags"].(map[string]any)
	if flags["backupEligible"] != true {
		t.Fatalf("JSON missing BackupEligible flag: %s", blob)
	}

	got := credFromRow(Cred{CredentialID: credID, PublicKey: blob, SignCount: 11})
	if string(got.ID) != string(credID) {
		t.Fatalf("id=%v want %v", got.ID, credID)
	}
	if !got.Flags.BackupEligible {
		t.Fatal("BackupEligible not restored from JSON public_key")
	}
	if got.Authenticator.SignCount != 11 {
		t.Fatalf("sign_count overlay: got %d want 11", got.Authenticator.SignCount)
	}
	if string(got.PublicKey) == string(blob) {
		t.Fatal("decoded PublicKey should be COSE bytes, not the JSON column")
	}
}

func TestCredFromRowRawByteFallback(t *testing.T) {
	cose := []byte{9, 8, 7, 6}
	id := []byte{1, 2, 3}
	got := credFromRow(Cred{
		CredentialID:    id,
		PublicKey:       cose,
		AttestationType: "none",
		Transport:       []string{"internal"},
		SignCount:       3,
	})
	if string(got.PublicKey) != string(cose) {
		t.Fatalf("fallback PublicKey=%v want raw COSE %v", got.PublicKey, cose)
	}
	if string(got.ID) != string(id) {
		t.Fatalf("id=%v want %v", got.ID, id)
	}
	if got.Flags.BackupEligible {
		t.Fatal("raw-byte fallback must not invent BackupEligible")
	}
	if got.Authenticator.SignCount != 3 {
		t.Fatalf("sign_count=%d want 3", got.Authenticator.SignCount)
	}
}

func TestPasskeyLoginFinishSetsSessionCookie(t *testing.T) {
	secret := []byte("test-secret-at-least-16")
	u := User{ID: "user-abc", Email: "a@example.com", Name: "Ada", Role: "admin"}
	app := fiber.New()
	h := &passkeyHandler{secret: secret, ttl: time.Hour}
	app.Post("/auth/passkey/login/finish", func(c fiber.Ctx) error {
		return h.issueSession(c, u)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/auth/passkey/login/finish", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			cookie = c
			break
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("missing %s cookie: %v", cookieName, resp.Header.Values("Set-Cookie"))
	}
	if !cookie.HttpOnly {
		t.Fatal("cc_session must be HttpOnly")
	}
	sess, err := ParseSession(secret, cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != u.ID {
		t.Fatalf("session user=%q want %q", sess.UserID, u.ID)
	}
}
