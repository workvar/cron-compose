package deploy

import (
	"regexp"
	"strings"
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
