package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestUnauthenticatedClearsSessionCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return unauthenticated(c, "unknown user")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "stale-session"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "unknown user") {
		t.Fatalf("body=%s", body)
	}

	got := resp.Header.Values("Set-Cookie")
	if len(got) == 0 {
		t.Fatal("expected Set-Cookie to clear the stale session")
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, cookieName+"=") {
		t.Fatalf("Set-Cookie=%q", joined)
	}
	if !strings.Contains(strings.ToLower(joined), "max-age=0") &&
		!strings.Contains(joined, "01 Jan 1970") &&
		!strings.Contains(joined, "Thu, 01 Jan 1970") {
		// Fiber may encode expiry in several ways; an expired cookie is enough.
		if !strings.Contains(joined, "Expires=") && !strings.Contains(strings.ToLower(joined), "max-age") {
			t.Fatalf("Set-Cookie did not expire the session: %q", joined)
		}
	}
}

func TestRequireAuthMissingCookie(t *testing.T) {
	app := fiber.New()
	app.Use(RequireAuth([]byte("test-secret-at-least-16"), nil, nil))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

func TestRequireAuthRejectsExpiredSession(t *testing.T) {
	secret := []byte("test-secret-at-least-16")
	expired := SignSession(secret, Session{UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour)})

	app := fiber.New()
	app.Use(RequireAuth(secret, nil, nil))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: expired})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
	if len(resp.Header.Values("Set-Cookie")) == 0 {
		t.Fatal("expected stale cookie to be cleared")
	}
}
