package api

import (
	"io"
	"net/http"
	"net/http/httptest"
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

// mountWeb bounces / into /app and reverse-proxies /app/* to the upstream with the
// path preserved.
func TestMountWebRedirectAndProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream:"+r.URL.Path)
	}))
	defer upstream.Close()

	app := fiber.New()
	mountWeb(app, upstream.URL)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("/ status=%d want=302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/app" {
		t.Errorf("/ location=%q want=/app", loc)
	}

	resp2, err := app.Test(httptest.NewRequest(http.MethodGet, "/app/jobs", nil), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("/app/jobs status=%d", resp2.StatusCode)
	}
	b, _ := io.ReadAll(resp2.Body)
	if string(b) != "upstream:/app/jobs" {
		t.Errorf("/app/jobs body=%q want upstream:/app/jobs", b)
	}
}

// With no upstream configured, mountWeb registers nothing.
func TestMountWebDisabled(t *testing.T) {
	app := fiber.New()
	mountWeb(app, "")
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("/ with no upstream status=%d want=404", resp.StatusCode)
	}
}
