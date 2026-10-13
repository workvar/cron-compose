package deploys

import (
	"regexp"
	"strings"
)

var processNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

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
// Keep in sync with agent/internal/deploy.QualifyProcessName.
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

// EnrichProcessNames fills SpecApp.ProcessName for API responses when the app
// did not already set an explicit override (ecosystem name, import, etc.).
func EnrichProcessNames(projectName string, apps []SpecApp) []SpecApp {
	if len(apps) == 0 {
		return apps
	}
	out := make([]SpecApp, len(apps))
	copy(out, apps)
	for i := range out {
		if strings.TrimSpace(out[i].ProcessName) != "" {
			continue
		}
		name := out[i].Name
		if name == "" {
			name = out[i].Root
		}
		out[i].ProcessName = QualifyProcessName(projectName, name)
	}
	return out
}

// ProcessNameForDeploy is the process_name sent to the agent on DeployCommand.
// For pm2, the default project-app qualification is omitted: an on-disk
// ecosystem.config.js apps[].name must supersede that computed token. Explicit
// names (ecosystem autopopulate, import, operator edit) are still sent as hints.
func ProcessNameForDeploy(project string, a SpecApp, pm string) string {
	n := strings.TrimSpace(a.ProcessName)
	if n == "" {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(pm), "pm2") {
		return n
	}
	label := a.Name
	if label == "" {
		label = a.Root
	}
	if n == QualifyProcessName(project, label) {
		return ""
	}
	return n
}
