package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadEnvFile applies KEY=VALUE lines from AGENT_ENV_FILE (or the platform default)
// to the process environment, without overriding anything already set. It runs
// before Load so the Windows service, which has no environment of its own, reads
// the same settings a systemd unit would. A missing file is not an error.
//
// Format: one KEY=VALUE per line, '#' comments, blank lines ignored, optional
// matching single or double quotes around the value.
func LoadEnvFile() error {
	path := strings.TrimSpace(os.Getenv("AGENT_ENV_FILE"))
	if path == "" {
		path = defaultEnvFile
	}
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	for k, v := range parseEnvFile(bufio.NewScanner(f)) {
		if _, set := os.LookupEnv(k); !set {
			_ = os.Setenv(k, v)
		}
	}
	return nil
}

func parseEnvFile(sc *bufio.Scanner) map[string]string {
	out := map[string]string{}
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			continue
		}
		out[k] = unquote(strings.TrimSpace(v))
	}
	return out
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}
