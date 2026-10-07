package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func findCompose(dir string) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// runtimeLanguage maps framework ids (nextjs, csharp, …) to a start-command bucket.
// Keep in sync with control-plane RuntimeLanguage — agent cannot import that package.
func runtimeLanguage(language string) string {
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
			return ""
		}
		return lang
	}
}

func startCommand(language, runScript, _ string) (string, []string) {
	if bin, args, ok := parseRunScript(runScript); ok {
		return bin, args
	}
	// Prefer an explicit run script from the framework preset; these are last-resort
	// fallbacks when language is set but run is empty.
	switch runtimeLanguage(language) {
	case "node", "bun":
		return "npm", []string{"start"}
	case "go":
		return "./app", nil
	case "python":
		return "python3", []string{"-m", "app"}
	case "dotnet":
		return "bash", []string{"-lc", "dotnet out/*.dll --urls http://0.0.0.0:8080"}
	default:
		return "", nil
	}
}

// parseRunScript turns a freeform run script into an executable + args.
// Shell operators (&&, |, ;, redirects) go through bash -lc.
func parseRunScript(runScript string) (string, []string, bool) {
	script := strings.TrimSpace(runScript)
	if script == "" {
		return "", nil, false
	}
	if strings.ContainsAny(script, "|&;<>") || strings.Contains(script, "&&") || strings.Contains(script, "||") {
		return "bash", []string{"-lc", script}, true
	}
	fields := strings.Fields(script)
	if len(fields) == 0 {
		return "", nil, false
	}
	return fields[0], fields[1:], true
}

func pm2StartArgs(name, language, eco, runScript string) []string {
	if eco != "" {
		return []string{"start", eco, "--update-env"}
	}
	bin, extra := startCommand(language, runScript, name)
	if bin == "" {
		return nil
	}
	args := []string{"start", bin, "--name", name}
	if len(extra) > 0 {
		args = append(args, "--")
		args = append(args, extra...)
	}
	return args
}

var unitNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func systemdUnitName(name string) string {
	n := unitNameRe.ReplaceAllString(name, "-")
	n = strings.Trim(n, "-")
	if n == "" {
		return "croncompose-app"
	}
	return n
}

func systemdUnit(name, work, execStart string, env map[string]string) string {
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=CronCompose " + name + "\nAfter=network.target\n\n")
	b.WriteString("[Service]\nType=simple\nWorkingDirectory=" + work + "\n")
	b.WriteString("ExecStart=" + execStart + "\nRestart=on-failure\n")
	for k, v := range env {
		b.WriteString("Environment=" + k + "=" + v + "\n")
	}
	b.WriteString("\n[Install]\nWantedBy=default.target\n")
	return b.String()
}

func (m *Manager) startProcess(ctx context.Context, runID, token, work, pm, language, runScript string, env map[string]string) error {
	name := filepath.Base(work)
	if language == "" {
		language = detectLangHint(work)
	}
	switch pm {
	case "", "none":
		m.logLine(runID, token, "process_manager=none; pick pm2, systemd, or docker on the project to host this app")
		return nil
	case "pm2":
		eco := findEcosystem(work)
		args := pm2StartArgs(name, language, eco, runScript)
		if len(args) == 0 {
			m.logLine(runID, token, "pm2: no start command; add a run script, ecosystem file, or set language to node")
			return nil
		}
		m.logLine(runID, token, "pm2 "+strings.Join(args, " "))
		return runCmd(ctx, work, env, "pm2", args...)
	case "docker":
		compose := findCompose(work)
		if compose == "" {
			m.logLine(runID, token, "no docker-compose file; add one or use Connectors")
			return nil
		}
		m.logLine(runID, token, "docker compose -f "+filepath.Base(compose)+" up -d")
		return runCmd(ctx, work, env, "docker", "compose", "-f", compose, "up", "-d")
	case "systemd":
		bin, args := startCommand(language, runScript, name)
		if bin == "" {
			m.logLine(runID, token, "systemd: no ExecStart guessed; set a run script or attach the unit from Connectors")
			return nil
		}
		execStart := bin
		if len(args) > 0 {
			execStart = bin + " " + strings.Join(args, " ")
		}
		unit := systemdUnitName(name)
		body := systemdUnit(name, work, execStart, env)
		dir := filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, unit+".service")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		m.logLine(runID, token, "systemd --user enable --now "+unit+".service")
		if err := runCmd(ctx, work, env, "systemctl", "--user", "daemon-reload"); err != nil {
			return fmt.Errorf("systemd reload: %w", err)
		}
		return runCmd(ctx, work, env, "systemctl", "--user", "enable", "--now", unit+".service")
	default:
		m.logLine(runID, token, "unknown process_manager="+pm)
		return nil
	}
}

func detectLangHint(work string) string {
	switch {
	case fileExists(filepath.Join(work, "package.json")):
		return "node"
	case fileExists(filepath.Join(work, "go.mod")):
		return "go"
	case fileExists(filepath.Join(work, "pyproject.toml")), fileExists(filepath.Join(work, "requirements.txt")):
		return "python"
	default:
		return ""
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
