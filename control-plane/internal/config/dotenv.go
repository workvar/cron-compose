package config

import (
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnvFromCWD finds a .env next to the process (cwd, then parents, then
// CRONCOMPOSE_ROOT) and applies it. Existing environment variables win, so a
// systemd unit or an already-sourced shell is never clobbered. Set CC_LOAD_DOTENV=0
// to skip (tests).
func loadDotEnvFromCWD() {
	if os.Getenv("CC_LOAD_DOTENV") == "0" {
		return
	}
	path := findDotEnv()
	if path == "" {
		return
	}
	_ = applyDotEnvFile(path)
}

func findDotEnv() string {
	if root := os.Getenv("CRONCOMPOSE_ROOT"); root != "" {
		p := filepath.Join(root, ".env")
		if fileExists(p) {
			return p
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 8; i++ {
		p := filepath.Join(wd, ".env")
		if fileExists(p) {
			return p
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}
	return ""
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func applyDotEnvFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	vals, err := parseDotEnv(string(b))
	if err != nil {
		return err
	}
	for k, v := range vals {
		if _, exists := os.LookupEnv(k); exists {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}

func parseDotEnv(content string) (map[string]string, error) {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
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
	return out, nil
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
