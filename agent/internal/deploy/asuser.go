package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// applyCredential puts a child process under run_as_user when the agent is allowed
// to switch. Nil cred is a no-op (run as the agent).
func applyCredential(cmd *exec.Cmd, cred *osuser.Credential) {
	if cred == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid:    cred.UID,
		Gid:    cred.GID,
		Groups: cred.Groups,
	}
}

// credEnvKeysDropped are inherited from the agent process but must not survive a
// user switch. HOME/USER come from Credential.Env; Go cache vars from a root
// systemd unit would otherwise keep pointing at /root while the child runs as pi.
var credEnvKeysDropped = []string{
	"HOME", "USER", "LOGNAME", "SHELL",
	"GOPATH", "GOMODCACHE", "GOCACHE", "GOTMPDIR",
	"TMPDIR", "TMP", "TEMP",
}

// credEnv merges identity overrides and optional TMPDIR for the target account.
func credEnv(cred *osuser.Credential, tmpDir string, extra map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	if cred != nil {
		env = stripEnvKeys(env, credEnvKeysDropped...)
		env = append(env, cred.Env()...)
	}
	if tmpDir != "" {
		env = stripEnvKeys(env, "TMPDIR", "TMP", "TEMP")
		env = append(env, "TMPDIR="+tmpDir, "TMP="+tmpDir, "TEMP="+tmpDir)
	}
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

func stripEnvKeys(env []string, keys ...string) []string {
	if len(keys) == 0 || len(env) == 0 {
		return env
	}
	drop := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		drop[k] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		k, _, ok := strings.Cut(e, "=")
		if ok {
			if _, skip := drop[k]; skip {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

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

// ensureUserDirs creates the deploy base and the account's ~/tmp (when set), then
// chowns them to the target user so later git/install steps can write as that user.
func ensureUserDirs(base, tmpDir string, cred *osuser.Credential) error {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	if tmpDir != "" {
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return err
		}
	}
	return chownTree(base, cred)
}

func chownTree(path string, cred *osuser.Credential) error {
	if cred == nil {
		return nil
	}
	// Absolute chown keeps us honest under ProtectSystem; -R covers nested releases.
	cmd := exec.Command("chown", "-R", fmt.Sprintf("%d:%d", cred.UID, cred.GID), path)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("chown %s: %s", path, msg)
	}
	return nil
}

// userTmpDir mirrors control-plane ClonePathForUser's tmp rule on the agent.
func userTmpDir(cred *osuser.Credential) string {
	if cred == nil || cred.Username == "" || cred.Username == "root" || cred.Home == "" {
		return "/tmp"
	}
	return filepath.Join(cred.Home, "tmp")
}
