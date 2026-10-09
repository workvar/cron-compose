package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// apiPrefixRewrite maps the public /api/* prefix onto /api/v1/* and leaves
// versioned and unrelated paths alone.
func TestAPIPrefixRewrite(t *testing.T) {
	app := fiber.New()
	app.Use(apiPrefixRewrite())
	app.Get("/api/v1/ping", func(c fiber.Ctx) error { return c.SendString("pong") })
	app.Get("/healthz", func(c fiber.Ctx) error { return c.SendString("ok") })

	cases := []struct {
		path string
		want int
		body string
	}{
		{"/api/ping", http.StatusOK, "pong"},    // public prefix rewritten to /api/v1
		{"/api/v1/ping", http.StatusOK, "pong"}, // versioned path untouched
		{"/healthz", http.StatusOK, "ok"},       // unrelated path untouched
		{"/api/missing", http.StatusNotFound, ""},
	}
	for _, tc := range cases {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, tc.path, nil))
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status=%d want=%d", tc.path, resp.StatusCode, tc.want)
		}
		if tc.body != "" {
			b, _ := io.ReadAll(resp.Body)
			if string(b) != tc.body {
				t.Errorf("%s: body=%q want=%q", tc.path, b, tc.body)
			}
		}
	}
}

// mountWeb serves the marketing landing at / (proxied to /app/landing) and
// reverse-proxies /app/* to the upstream with the path preserved.
func TestMountWebLandingAndProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream:"+r.URL.Path)
	}))
	defer upstream.Close()

	app := fiber.New()
	mountWeb(app, upstream.URL)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/ status=%d want=200", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "upstream:/app/landing" {
		t.Fatalf("/ body=%q want upstream:/app/landing", b)
	}

	resp2, err := app.Test(httptest.NewRequest(http.MethodGet, "/app/jobs", nil), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("/app/jobs status=%d", resp2.StatusCode)
	}
	b2, _ := io.ReadAll(resp2.Body)
	if string(b2) != "upstream:/app/jobs" {
		t.Fatalf("/app/jobs body=%q want upstream:/app/jobs", b2)
	}

	for _, tc := range []struct{ path, wantLoc string }{
		{"/docs", "/app/docs"},
		{"/docs/guide", "/app/docs/guide"},
		{"/use-cases", "/app/use-cases"},
		{"/use-cases/fleet", "/app/use-cases/fleet"},
	} {
		r, err := app.Test(httptest.NewRequest(http.MethodGet, tc.path, nil), fiber.TestConfig{Timeout: 5 * time.Second})
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if r.StatusCode != http.StatusFound {
			t.Fatalf("%s status=%d want=302", tc.path, r.StatusCode)
		}
		if loc := r.Header.Get("Location"); loc != tc.wantLoc {
			t.Fatalf("%s Location=%q want %q", tc.path, loc, tc.wantLoc)
		}
	}
}

// With no upstream configured, mountWeb serves an nginx-style welcome page at /.
func TestMountWebDisabled(t *testing.T) {
	app := fiber.New()
	mountWeb(app, "")
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/ with no upstream status=%d want=200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("/ content-type=%q want text/html", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "Welcome to CronCompose!") {
		t.Errorf("/ body=%q want welcome page", b)
	}
	if !strings.Contains(body, `class="btn"`) || !strings.Contains(body, `href="/app/"`) {
		t.Errorf("/ body missing control-plane button, got %q", b)
	}
}
