// This file makes the control plane the single public entry point: it serves the
// Next.js UI under /app and exposes the public /api prefix, so the bare domain
// fronts both the app and the REST API without a separate reverse proxy.
package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/proxy"
)

const (
	apiPublicPrefix   = "/api"    // what browsers call
	apiInternalPrefix = "/api/v1" // where the versioned routes actually live
)

// apiPrefixRewrite maps the public REST prefix (/api/*) onto the control plane's
// versioned routes (/api/v1/*). Browsers fetch /api/...; server-side UI reads use
// /api/v1 directly and pass through untouched. Mounted before the route groups so
// the rewritten path is what gets matched.
func apiPrefixRewrite() fiber.Handler {
	return func(c fiber.Ctx) error {
		p := c.Path()
		if strings.HasPrefix(p, apiPublicPrefix+"/") &&
			p != apiInternalPrefix &&
			!strings.HasPrefix(p, apiInternalPrefix+"/") {
			c.Path(apiInternalPrefix + strings.TrimPrefix(p, apiPublicPrefix))
		}
		return c.Next()
	}
}

// mountWeb reverse-proxies the Next.js UI under /app and bounces the bare root into
// it. upstream is the internal Next.js address (e.g. http://web:3000); when empty,
// the UI is served elsewhere and these routes are skipped.
func mountWeb(app *fiber.App, upstream string) {
	if upstream == "" {
		return
	}
	upstream = strings.TrimRight(upstream, "/")

	app.Get("/", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusFound).To("/app")
	})
	// Forward /app and everything under it to Next, preserving path + query. The UI
	// uses basePath:/app, so all its assets and routes already live under this prefix.
	app.Use("/app", func(c fiber.Ctx) error {
		return proxy.Do(c, upstream+c.OriginalURL())
	})
}
