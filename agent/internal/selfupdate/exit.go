package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ClearPinnedAgentVersion removes a systemd pin of AGENT_VERSION so the next
// process start reports the binary's linked build version. Installers used to
// bake Environment=AGENT_VERSION into the unit; after a self-update that pin
// kept Hello on the old tag and the UI stuck on "restarting".
//
// Best-effort: no-op when not root or when no unit directory is writable.
func ClearPinnedAgentVersion() {
	if os.Geteuid() != 0 {
		return
	}
	unit := strings.TrimSpace(os.Getenv("CC_AGENT_UNIT"))
	if unit == "" {
		unit = "croncompose-agent.service"
	}
	dir := filepath.Join("/etc/systemd/system", unit+".d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, "unset-agent-version.conf")
	if err := os.WriteFile(path, []byte("[Service]\nUnsetEnvironment=AGENT_VERSION\n"), 0o644); err != nil {
		return
	}
	systemctl := strings.TrimSpace(os.Getenv("CC_PRIVCTL_SYSTEMCTL"))
	if systemctl == "" {
		if p, err := exec.LookPath("systemctl"); err == nil {
			systemctl = p
		}
	}
	if systemctl == "" {
		return
	}
	_ = exec.Command(systemctl, "daemon-reload").Run()
}

// FlushAndExit gives the progress stream a moment to drain, then exits 0 so the
// supervisor restarts on the new binary.
func FlushAndExit() {
	time.Sleep(300 * time.Millisecond)
	os.Exit(0)
}
