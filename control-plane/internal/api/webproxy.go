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
// / still serves a small nginx-style welcome page so the HTTP port is not a 404.
func mountWeb(app *fiber.App, upstream string) {
	if upstream == "" {
		app.Get("/", welcomeRoot)
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

const welcomeHTML = `<!DOCTYPE html>
<html>
<head>
<title>Welcome to CronCompose!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
.btn {
  display: inline-block;
  margin: 0.75em 0 0.25em;
  padding: 0.45em 1.1em;
  color: inherit;
  text-decoration: none;
  border: 1px solid currentColor;
  border-radius: 3px;
  transition: transform 160ms ease-out;
}
.btn:active { transform: scale(0.97); }
@media (hover: hover) and (pointer: fine) {
  .btn:hover { opacity: 0.85; }
}
</style>
</head>
<body>
<h1>Welcome to CronCompose!</h1>
<p>If you see this page, the web server is successfully installed and
working. Further configuration is required.</p>

<p>The control plane UI is at <a href="/app/">/app/</a>.<br/>
REST API is at <a href="/api/">/api/</a>.</p>

<p><a class="btn" href="/app/">Open control plane</a></p>

<p><em>Thank you for using CronCompose.</em></p>
</body>
</html>
`

func welcomeRoot(c fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).SendString(welcomeHTML)
}
