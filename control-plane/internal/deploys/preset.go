package deploys

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Detection is the Vercel-style guess we make from a repo's file tree.
type Detection struct {
	Language        string   `json:"language"`
	InstallScript   string   `json:"install_script"`
	HasPM2Ecosystem bool     `json:"has_pm2_ecosystem"`
	SupportsPort    bool     `json:"supports_port"`
	Workspaces      []string `json:"workspaces"`
	SuggestedRoot   string   `json:"root_directory"`
}

// Detect inspects a map of path -> file contents (root-relative).
func Detect(files map[string]string) Detection {
	d := Detection{Language: "unknown", SuggestedRoot: ".", InstallScript: ""}
	names := make([]string, 0, len(files))
	for p := range files {
		names = append(names, p)
	}

	has := func(name string) bool {
		_, ok := files[name]
		return ok
	}
	anyHas := func(suffixes ...string) bool {
		for _, p := range names {
			base := path.Base(p)
			for _, s := range suffixes {
				if base == s {
					return true
				}
			}
		}
		return false
	}

	pkgJSON := files["package.json"]
	switch {
	case pkgJSON != "" || anyHas("package.json"):
		d.Language = "node"
		d.SupportsPort = true
		lockPnpm := has("pnpm-lock.yaml") || has("pnpm-workspace.yaml")
		lockYarn := has("yarn.lock")
		lockBun := has("bun.lockb") || has("bun.lock")
		switch {
		case lockPnpm:
			d.InstallScript = "pnpm install"
		case lockYarn:
			d.InstallScript = "yarn install"
		case lockBun:
			d.InstallScript = "bun install"
		default:
			d.InstallScript = "npm install"
		}
		d.Workspaces = detectWorkspaces(files)
		if looksLikeBuildableJS(pkgJSON, files) || len(d.Workspaces) > 0 {
			d.InstallScript += " && " + jsBuildCommand(lockPnpm, lockYarn, lockBun)
		}
	case has("requirements.txt") || has("pyproject.toml") || has("Pipfile"):
		d.Language = "python"
		d.SupportsPort = true
		if has("requirements.txt") {
			d.InstallScript = "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt"
		} else {
			d.InstallScript = "python3 -m venv .venv && . .venv/bin/activate && pip install -e ."
		}
	case has("go.mod"):
		d.Language = "go"
		d.InstallScript = "go build -o app ."
	case has("Cargo.toml"):
		d.Language = "rust"
		d.InstallScript = "cargo build --release"
	case has("Gemfile"):
		d.Language = "ruby"
		d.InstallScript = "bundle install"
	case has("composer.json"):
		d.Language = "php"
		d.InstallScript = "composer install"
	case has("mix.exs"):
		d.Language = "elixir"
		d.InstallScript = "mix deps.get && mix compile"
	case has("pom.xml") || has("build.gradle") || has("build.gradle.kts"):
		d.Language = "java"
		if has("pom.xml") {
			d.InstallScript = "mvn -q package"
		} else {
			d.InstallScript = "./gradlew build"
		}
	case has("docker-compose.yml") || has("docker-compose.yaml") || has("compose.yml") || has("compose.yaml"):
		d.Language = "docker"
		d.InstallScript = "docker compose up -d"
	}

	d.HasPM2Ecosystem = anyHas("ecosystem.config.js", "ecosystem.config.cjs", "ecosystem.config.mjs", "ecosystem.config.ts", "pm2.json")
	if d.HasPM2Ecosystem && d.Language == "unknown" {
		d.Language = "node"
	}
	if d.Language == "node" {
		d.SupportsPort = true
	}
	for _, body := range files {
		if strings.Contains(body, "process.env.PORT") || strings.Contains(body, "PORT=") {
			d.SupportsPort = true
			break
		}
	}
	if d.Workspaces == nil {
		d.Workspaces = []string{}
	}
	return d
}

func looksLikeBuildableJS(pkgJSON string, files map[string]string) bool {
	if strings.Contains(pkgJSON, `"next"`) || strings.Contains(pkgJSON, `"vite"`) ||
		strings.Contains(pkgJSON, `"nuxt"`) || strings.Contains(pkgJSON, `"remix"`) {
		return true
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	_ = json.Unmarshal([]byte(pkgJSON), &pkg)
	if pkg.Scripts["build"] != "" {
		return true
	}
	_, hasTS := files["tsconfig.json"]
	return hasTS
}

func jsBuildCommand(pnpm, yarn, bun bool) string {
	switch {
	case pnpm:
		return "pnpm run build"
	case yarn:
		return "yarn build"
	case bun:
		return "bun run build"
	default:
		return "npm run build"
	}
}

func detectWorkspaces(files map[string]string) []string {
	seen := map[string]struct{}{}
	var pkg struct {
		Workspaces any `json:"workspaces"`
	}
	_ = json.Unmarshal([]byte(files["package.json"]), &pkg)

	for p := range files {
		if path.Base(p) != "package.json" || p == "package.json" {
			continue
		}
		dir := path.Dir(p)
		if dir == "." || dir == "" {
			continue
		}
		seen[dir] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// DefaultLanguagePaths is the Vercel-style per-language clone root.
func DefaultLanguagePaths() map[string]string {
	return map[string]string{
		"node":    "/opt/apps/node",
		"python":  "/opt/apps/python",
		"go":      "/opt/apps/go",
		"rust":    "/opt/apps/rust",
		"ruby":    "/opt/apps/ruby",
		"php":     "/opt/apps/php",
		"elixir":  "/opt/apps/elixir",
		"java":    "/opt/apps/java",
		"docker":  "/opt/apps/docker",
		"unknown": "/opt/apps",
	}
}

// ClonePath joins the language root with a sanitized repo directory name.
func ClonePath(paths map[string]string, language, repoFullName string) string {
	base := ""
	if paths != nil {
		base = paths[language]
	}
	if base == "" {
		base = DefaultLanguagePaths()[language]
	}
	if base == "" {
		base = "/opt/apps"
	}
	return strings.TrimRight(base, "/") + "/" + repoDirName(repoFullName)
}

var unsafeDir = regexp.MustCompile(`[^a-z0-9]+`)

func repoDirName(full string) string {
	name := full
	if i := strings.LastIndex(full, "/"); i >= 0 {
		name = full[i+1:]
	}
	name = strings.ToLower(strings.TrimSpace(name))
	name = unsafeDir.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "app"
	}
	return name
}
