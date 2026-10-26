package executor

import (
	"os"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	"github.com/croncompose/croncompose/agent/internal/platform"
)

// limited reports whether any resource cap is set.
func (j Job) limited() bool {
	return j.CPUQuotaPercent > 0 || j.MemoryMaxMB > 0 || j.TasksMax > 0 || j.IOWeight > 0
}

// buildEnv assembles the child's environment.
//
// The agent deliberately does NOT hand the job its own environment wholesale: an agent
// started by pm2 or systemd carries a pile of variables a job has no business seeing,
// including anything the operator exported for the agent itself. What a job gets is a
// small, predictable base plus whatever the job declares, plus the identity of the user
// it runs as.
func buildEnv(envMap map[string]string, cred *osuser.Credential) []string {
	base := map[string]string{"PATH": pathOrDefault()}
	for _, k := range platform.PassthroughEnv() {
		base[k] = os.Getenv(k)
	}
	for _, kv := range platform.ExtraEnv() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			base[k] = v
		}
	}
	// Running as another user replaces the identity variables wholesale.
	for _, kv := range cred.Env() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			base[k] = v
		}
	}
	// The job's own env wins over everything, including PATH, so a job can pin its
	// toolchain.
	for k, v := range envMap {
		base[k] = v
	}

	out := make([]string, 0, len(base))
	for k, v := range base {
		if v == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func pathOrDefault() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return defaultPath
}
