package deploy

import (
	"strings"
)

// splitInstallBuild splits an install script into dependency-install vs build
// commands so flow waves can order them across apps (build A after install B).
func splitInstallBuild(script, language string) (install, build string) {
	parts := splitShellCommands(script)
	if len(parts) == 0 {
		return "", ""
	}
	var installParts, buildParts []string
	for _, cmd := range parts {
		switch classifyInstallKind(cmd, language) {
		case "install":
			installParts = append(installParts, cmd)
		case "build":
			buildParts = append(buildParts, cmd)
		default:
			// Unclassified pieces stay with build if we already have install deps,
			// otherwise with install (single opaque script).
			if len(installParts) > 0 {
				buildParts = append(buildParts, cmd)
			} else {
				installParts = append(installParts, cmd)
			}
		}
	}
	if len(installParts) == 0 && len(buildParts) == 0 {
		return strings.TrimSpace(script), ""
	}
	if len(installParts) == 0 {
		return "", strings.Join(buildParts, " && ")
	}
	return strings.Join(installParts, " && "), strings.Join(buildParts, " && ")
}

func splitShellCommands(script string) []string {
	trimmed := strings.TrimSpace(script)
	if trimmed == "" {
		return nil
	}
	var raw []string
	if strings.Contains(trimmed, "&&") {
		raw = strings.Split(trimmed, "&&")
	} else {
		raw = strings.Split(trimmed, "\n")
	}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func classifyInstallKind(cmd, language string) string {
	c := strings.ToLower(strings.TrimSpace(cmd))
	switch {
	case matchPrefix(c, "npm ci", "npm install", "npm i ", "pnpm install", "pnpm i ", "yarn install", "yarn i ", "bun install", "bun i "),
		matchPrefix(c, "go mod download", "go mod tidy", "go mod vendor"),
		matchPrefix(c, "pip install", "pip3 install", "pipenv install", "poetry install", "uv sync", "uv add"),
		matchPrefix(c, "bundle install", "cargo fetch"):
		return "install"
	case strings.Contains(c, "build") || strings.Contains(c, "compile"),
		matchPrefix(c, "go build", "cargo build", "tsc", "vite", "webpack", "next", "nuxt", "astro"),
		matchPrefix(c, "npm run build", "pnpm run build", "pnpm build", "yarn run build", "yarn build"):
		return "build"
	default:
		_ = language
		return "command"
	}
}

func matchPrefix(c string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(c, p) || c == strings.TrimSpace(p) {
			return true
		}
	}
	return false
}
