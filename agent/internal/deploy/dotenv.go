package deploy

import (
	"os"
	"path/filepath"
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

// applyListenPort sets PORT and overwrites listen aliases already present in env
// so a UI port of 3004 updates PROXY_PORT=80 from .env rather than leaving the
// process bound to a privileged port.
func applyListenPort(env map[string]string, port int32) {
	if env == nil || port <= 0 {
		return
	}
	p := strconv.Itoa(int(port))
	env["PORT"] = p
	for _, k := range listenPortAliases {
		if _, ok := env[k]; ok {
			env[k] = p
		}
	}
}
