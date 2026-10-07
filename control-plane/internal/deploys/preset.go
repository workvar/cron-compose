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

// DetectAt scopes Detect to one subfolder of the repo, so a monorepo block (say,
// a Go API under backend/) is detected from its own files rather than the whole
// tree. Without this, a package.json anywhere in the repo (even in a sibling
// frontend/) made Detect flag every block as Node, because Detect's own anyHas
// check matches a filename at any depth. root == "" or "." runs unscoped, same as
// Detect(files).
func DetectAt(files map[string]string, root string) Detection {
	root = strings.Trim(strings.TrimSpace(root), "/")
	if root == "" || root == "." {
		return Detect(files)
	}
	prefix := root + "/"
	scoped := make(map[string]string, len(files))
	for p, body := range files {
		if rel, ok := strings.CutPrefix(p, prefix); ok && rel != "" {
			scoped[rel] = body
		}
	}
	return Detect(scoped)
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
		d.Language = detectJSFramework(pkgJSON, files)
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
		d.Language = detectPythonFramework(files)
		d.SupportsPort = true
		if has("requirements.txt") {
			d.InstallScript = "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt"
		} else {
			d.InstallScript = "python3 -m venv .venv && . .venv/bin/activate && pip install -e ."
		}
	case has("go.mod"):
		d.Language = "go"
		d.InstallScript = "go build -o app ."
	case has("global.json") || hasExtension(names, ".csproj", ".fsproj", ".vbproj", ".sln"):
		d.Language = "dotnet"
		d.SupportsPort = true
		d.InstallScript = "dotnet restore && dotnet publish -c Release -o out"
	case has("Cargo.toml"):
		d.Language = "rust"
		d.InstallScript = "cargo build --release"
	case has("Gemfile"):
		d.Language = detectRubyFramework(files)
		d.InstallScript = "bundle install"
		if d.Language == "rails" {
			d.InstallScript += " && bundle exec rake assets:precompile"
			d.SupportsPort = true
		}
	case has("composer.json"):
		d.Language = detectPHPFramework(files)
		d.InstallScript = "composer install"
		if d.Language == "laravel" {
			d.InstallScript = "composer install --no-dev --optimize-autoloader"
			d.SupportsPort = true
		}
	case has("mix.exs"):
		d.Language = "elixir"
		d.InstallScript = "mix deps.get && mix compile"
	case has("pom.xml") || has("build.gradle") || has("build.gradle.kts"):
		d.Language = detectJavaFramework(files)
		if has("pom.xml") {
			d.InstallScript = "mvn -q -DskipTests package"
		} else {
			d.InstallScript = "./gradlew build"
		}
		d.SupportsPort = true
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
		strings.Contains(pkgJSON, `"nuxt"`) || strings.Contains(pkgJSON, `"remix"`) ||
		strings.Contains(pkgJSON, `"@nestjs/core"`) || strings.Contains(pkgJSON, `"astro"`) ||
		strings.Contains(pkgJSON, `"@sveltejs/kit"`) {
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

// detectJSFramework picks a specific stack id from package.json when possible.
func detectJSFramework(pkgJSON string, files map[string]string) string {
	switch {
	case strings.Contains(pkgJSON, `"next"`):
		return "nextjs"
	case strings.Contains(pkgJSON, `"@nestjs/core"`):
		return "nestjs"
	case strings.Contains(pkgJSON, `"nuxt"`):
		return "nuxt"
	case strings.Contains(pkgJSON, `"@remix-run/"`) || strings.Contains(pkgJSON, `"remix"`):
		return "remix"
	case strings.Contains(pkgJSON, `"@sveltejs/kit"`):
		return "sveltekit"
	case strings.Contains(pkgJSON, `"astro"`):
		return "astro"
	case strings.Contains(pkgJSON, `"vue"`) && (strings.Contains(pkgJSON, `"vite"`) || hasFile(files, "vite.config.ts", "vite.config.js", "vite.config.mjs")):
		return "vue"
	case strings.Contains(pkgJSON, `"react"`) && !strings.Contains(pkgJSON, `"next"`) &&
		(strings.Contains(pkgJSON, `"vite"`) || hasFile(files, "vite.config.ts", "vite.config.js", "vite.config.mjs")):
		return "react"
	case strings.Contains(pkgJSON, `"express"`):
		return "express"
	case hasFile(files, "bun.lockb", "bun.lock"):
		return "bun"
	default:
		return "node"
	}
}

func detectPythonFramework(files map[string]string) string {
	blob := strings.ToLower(files["requirements.txt"] + "\n" + files["pyproject.toml"] + "\n" + files["Pipfile"])
	switch {
	case strings.Contains(blob, "fastapi") || strings.Contains(blob, "uvicorn"):
		return "fastapi"
	case strings.Contains(blob, "django"):
		return "django"
	case strings.Contains(blob, "flask"):
		return "flask"
	default:
		return "python"
	}
}

func detectRubyFramework(files map[string]string) string {
	gem := files["Gemfile"]
	if strings.Contains(gem, "rails") || hasFile(files, "config/application.rb", "bin/rails") {
		return "rails"
	}
	return "ruby"
}

func detectPHPFramework(files map[string]string) string {
	if hasFile(files, "artisan") || strings.Contains(files["composer.json"], `"laravel/framework"`) {
		return "laravel"
	}
	return "php"
}

func detectJavaFramework(files map[string]string) string {
	blob := files["pom.xml"] + files["build.gradle"] + files["build.gradle.kts"]
	if strings.Contains(blob, "spring-boot") || strings.Contains(blob, "org.springframework.boot") {
		return "spring"
	}
	return "java"
}

func hasFile(files map[string]string, names ...string) bool {
	for _, n := range names {
		if _, ok := files[n]; ok {
			return true
		}
	}
	return false
}

func hasExtension(paths []string, exts ...string) bool {
	for _, p := range paths {
		lower := strings.ToLower(p)
		for _, ext := range exts {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
	}
	return false
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
// Framework ids (nextjs, nestjs, …) resolve through RuntimeLanguage first.
func DefaultLanguagePaths() map[string]string {
	return map[string]string{
		"node":    "/opt/apps/node",
		"python":  "/opt/apps/python",
		"go":      "/opt/apps/go",
		"rust":    "/opt/apps/rust",
		"dotnet":  "/opt/apps/dotnet",
		"ruby":    "/opt/apps/ruby",
		"php":     "/opt/apps/php",
		"elixir":  "/opt/apps/elixir",
		"java":    "/opt/apps/java",
		"docker":  "/opt/apps/docker",
		"deno":    "/opt/apps/deno",
		"bun":     "/opt/apps/bun",
		"unknown": "/opt/apps",
	}
}

// RuntimeLanguage maps a framework id to the clone-path / agent runtime bucket.
func RuntimeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "nextjs", "nestjs", "react", "vue", "nuxt", "remix", "sveltekit", "astro",
		"express", "typescript", "javascript", "nodejs":
		return "node"
	case "fastapi", "django", "flask":
		return "python"
	case "csharp", "aspnet", "aspnetcore":
		return "dotnet"
	case "rails":
		return "ruby"
	case "laravel":
		return "php"
	case "spring", "kotlin":
		return "java"
	case "golang":
		return "go"
	default:
		lang := strings.ToLower(strings.TrimSpace(language))
		if lang == "" {
			return "unknown"
		}
		return lang
	}
}

// ClonePath joins the language root with a sanitized repo directory name.
func ClonePath(paths map[string]string, language, repoFullName string) string {
	runtime := RuntimeLanguage(language)
	base := ""
	if paths != nil {
		base = paths[runtime]
		if base == "" {
			base = paths[language]
		}
	}
	if base == "" {
		base = DefaultLanguagePaths()[runtime]
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
