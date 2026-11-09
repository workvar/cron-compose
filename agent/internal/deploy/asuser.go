package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	"github.com/croncompose/croncompose/agent/internal/platform"
)

// applyCredential puts a child process under run_as_user when the agent is allowed
// to switch. Nil cred is a no-op (run as the agent).
func applyCredential(cmd *exec.Cmd, cred *osuser.Credential) {
	osuser.Apply(cmd, cred)
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
// Extra keys replace any prior value: appending duplicates left getenv() seeing
// the agent's stale FRONTEND_PORT / PATH ahead of the deploy overlay.
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
	if len(extra) > 0 {
		keys := make([]string, 0, len(extra))
		for k := range extra {
			keys = append(keys, k)
		}
		env = stripEnvKeys(env, keys...)
		for k, v := range extra {
			env = append(env, k+"="+v)
		}
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

// resolveBin finds name on the deploy user's login PATH (nvm/fnm/etc.), then falls
// back to the agent process PATH. Absolute paths are returned unchanged. Callers
// that exec the result should also pass pathWithBinDir so siblings like npm next
// to pm2 stay visible to child processes.
func resolveBin(cred *osuser.Credential, name string) string {
	if name == "" || filepath.IsAbs(name) {
		return name
	}
	if path, err := lookPathAs(cred, name); err == nil {
		return path
	}
	if path, err := platform.LookPath(name); err == nil {
		return path
	}
	return name
}

// pathWithBinDir prepends the directory of an absolute binary to PATH so tools
// installed beside it (e.g. npm next to pm2 under nvm) remain findable when the
// agent process PATH is a minimal systemd default.
func pathWithBinDir(env map[string]string, binPath string) map[string]string {
	if !filepath.IsAbs(binPath) {
		return env
	}
	dir := filepath.Dir(binPath)
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	base := out["PATH"]
	if base == "" {
		base = os.Getenv("PATH")
	}
	sep := string(os.PathListSeparator)
	if base == "" {
		out["PATH"] = dir
		return out
	}
	if base == dir || strings.HasPrefix(base, dir+sep) {
		out["PATH"] = base
		return out
	}
	for _, p := range strings.Split(base, sep) {
		if p == dir {
			out["PATH"] = base
			return out
		}
	}
	out["PATH"] = dir + sep + base
	return out
}

// ensureUserDirs creates the deploy base and the account's ~/tmp (when set), and
// hands the base directory itself to the target user.
//
// This used to chown the base recursively, which walked every kept release and
// its node_modules before preflight printed a single line. Release trees are
// chowned when they are created, so only the top-level directories need it here.
func ensureUserDirs(base, tmpDir string, cred *osuser.Credential) error {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	if tmpDir != "" {
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return err
		}
	}
	return chownShallow(base, cred)
}

// chownLayout hands the layout's top-level entries to the deploy user after a
// swap: the base, releases/, the current symlink itself, and the shared .env.
// Release contents were chowned when the release was built.
func chownLayout(layout Layout, cred *osuser.Credential) {
	for _, p := range []string{layout.Base, layout.Releases, layout.Current, filepath.Join(layout.Base, ".env")} {
		_ = chownShallow(p, cred)
	}
}

// userTmpDir mirrors control-plane ClonePathForUser's tmp rule on the agent.
func userTmpDir(cred *osuser.Credential) string {
	if cred == nil || cred.Username == "" || cred.Username == "root" || cred.Home == "" {
		return platform.TempDir()
	}
	return filepath.Join(cred.Home, "tmp")
}
