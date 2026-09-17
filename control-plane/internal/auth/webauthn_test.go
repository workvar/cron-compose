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
