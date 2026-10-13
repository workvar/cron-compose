package deploy

import (
	"regexp"
	"strings"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

var processNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// slugProcessName turns a freeform label into a pm2/systemd-safe token.
func slugProcessName(s string) string {
	n := processNameRe.ReplaceAllString(strings.TrimSpace(s), "-")
	n = strings.Trim(n, "-")
	for strings.Contains(n, "--") {
		n = strings.ReplaceAll(n, "--", "-")
	}
	return strings.ToLower(n)
}

// QualifyProcessName prefixes the app process name with the project name so two
// projects that both ship a "web" app do not collide in pm2/systemd.
//
//	QualifyProcessName("shop", "web") → "shop-web"
//	QualifyProcessName("shop", "shop") → "shop"   (single-app / same name)
//	QualifyProcessName("shop", "shop-web") → "shop-web"  (already qualified)
func QualifyProcessName(project, app string) string {
	p := slugProcessName(project)
	a := slugProcessName(app)
	if p == "" {
		if a == "" {
			return "app"
		}
		return a
	}
	if a == "" || a == p {
		return p
	}
	prefix := p + "-"
	if strings.HasPrefix(a, prefix) {
		return a
	}
	return prefix + a
}

// ProcessNameFor returns the host process name for an app: an explicit
// process_name from the control plane (ecosystem override) wins; otherwise
// QualifyProcessName(project, app name).
func ProcessNameFor(project string, app *agentv1.DeployApp) string {
	if app != nil {
		if n := strings.TrimSpace(app.GetProcessName()); n != "" {
			return n
		}
		name := app.GetName()
		if name == "" {
			name = "app"
		}
		return QualifyProcessName(project, name)
	}
	return QualifyProcessName(project, "app")
}
