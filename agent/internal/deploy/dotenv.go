package deploy

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// listenPortAliases are app-specific listen-port env keys. When the operator sets
// the UI PORT field we keep these in sync if they already exist (e.g. from a
// workdir .env), so apps that read PROXY_PORT instead of PORT still bind where
// CronCompose expects.
var listenPortAliases = []string{
	"PROXY_PORT",
	"HTTP_PORT",
	"APP_PORT",
	"LISTEN_PORT",
	"SERVER_PORT",
}

// mergeDotEnvAround fills env with keys from .env files walking from work up to
// a few parents. Existing keys in env win (deploy / UI overlay). Missing files
// are ignored.
func mergeDotEnvAround(work string, env map[string]string) map[string]string {
	if env == nil {
		env = map[string]string{}
	}
	if strings.TrimSpace(work) == "" {
		return env
	}
	dir := work
	for i := 0; i < 6; i++ {
		path := filepath.Join(dir, ".env")
		if vals := readDotEnvFile(path); len(vals) > 0 {
			for k, v := range vals {
				if _, ok := env[k]; ok {
					continue
				}
				env[k] = v
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return env
}

func readDotEnvFile(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseDotEnv(string(b))
}

func parseDotEnv(content string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "export ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
			line = trimmed
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = unquoteEnv(strings.TrimSpace(v))
	}
	return out
}

func unquoteEnv(v string) string {
	if n := len(v); n >= 2 {
		if v[0] == '"' && v[n-1] == '"' {
			inner := v[1 : n-1]
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			return inner
		}
		if v[0] == '\'' && v[n-1] == '\'' {
			return v[1 : n-1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// applyListenPort sets PORT from the UI/app port. For a single-listen app that
// only has PROXY_PORT (etc.) in .env, those aliases are synced so a UI port
// change moves the process off a stale privileged port. Monorepos that also
// define FRONTEND_PORT / BACKEND_PORT keep those values — rewriting PROXY_PORT
// to the frontend's 3002 while starting peepal-frontend corrupted shared env.
func applyListenPort(env map[string]string, port int32) {
	if env == nil || port <= 0 {
		return
	}
	p := strconv.Itoa(int(port))
	env["PORT"] = p
	if _, ok := env["FRONTEND_PORT"]; ok {
		return
	}
	if _, ok := env["BACKEND_PORT"]; ok {
		return
	}
	for _, k := range listenPortAliases {
		if _, ok := env[k]; ok {
			env[k] = p
		}
	}
}

// writeDotEnvFile writes KEY=value lines for a release or project base. Empty
// keys are skipped. Values are quoted when they need it.
func writeDotEnvFile(path string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# Written by CronCompose deploy — do not edit during a run\n")
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(quoteDotEnv(env[k]))
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func quoteDotEnv(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t\n\r\"'#") || strings.Contains(v, "=") {
		esc := strings.ReplaceAll(v, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		return `"` + esc + `"`
	}
	return v
}

