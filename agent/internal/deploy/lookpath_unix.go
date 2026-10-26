//go:build !windows

package deploy

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// lookPathAs asks a login shell of the target user whether bin is on PATH. This is
// what makes nvm/fnm installs under ~pi visible to preflight even though the agent
// process itself has a minimal systemd PATH. Uses osuser.WrapScript because plain
// `bash -lc` is non-interactive and skips the nvm block in ~/.bashrc.
func lookPathAs(cred *osuser.Credential, bin string) (string, error) {
	if bin == "" || strings.ContainsAny(bin, " \t\n`$\\\"'") {
		return "", fmt.Errorf("invalid binary name")
	}
	script := osuser.WrapScript("command -v " + bin)
	cmd := exec.Command("/bin/bash", "-lc", script)
	cmd.Env = credEnv(cred, "", nil)
	applyCredential(cmd, cred)
	out, err := cmd.Output()
	path := strings.TrimSpace(string(out))
	if err != nil || path == "" {
		return "", fmt.Errorf("%s not found", bin)
	}
	return path, nil
}
