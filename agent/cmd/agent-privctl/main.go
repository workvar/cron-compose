// Command agent-privctl is the root-owned helper that elevates or demotes the
// CronCompose agent via a systemd drop-in. The agent invokes it only as:
//
//	sudo -n /usr/libexec/croncompose/agent-privctl elevate
//	sudo -n /usr/libexec/croncompose/agent-privctl demote
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultUnit     = "croncompose-agent.service"
	pm2SystemdErr   = "systemd required to run agent as root under pm2"
	serviceUserFile = "agent-service-user"
	supervisorFile  = "agent-supervisor"
	dropInName      = "root.conf"
	defaultDataDir  = "/var/lib/croncompose"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func validateArgs(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-privctl elevate|demote")
	}
	switch args[0] {
	case "elevate", "demote":
		return nil
	default:
		return fmt.Errorf("usage: agent-privctl elevate|demote")
	}
}

func run(args []string) error {
	if err := validateArgs(args); err != nil {
		return err
	}
	dataDir := resolveDataDir()
	systemctl := systemctlBin()
	supervisor := readMarker(dataDir, supervisorFile)
	if supervisor == "pm2" || systemctl == "" || !isExecutable(systemctl) {
		return fmt.Errorf("%s", pm2SystemdErr)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, ".run"), 0o755); err != nil {
		return fmt.Errorf("create marker dir: %w", err)
	}
	switch args[0] {
	case "elevate":
		return elevate(dataDir, systemctl)
	default:
		return demote(dataDir, systemctl)
	}
}

func elevate(dataDir, systemctl string) error {
	if err := ensureServiceUser(dataDir); err != nil {
		return err
	}
	if err := writeMarker(dataDir, supervisorFile, "systemd"); err != nil {
		return err
	}
	dropIn := dropInFile()
	if err := os.MkdirAll(filepath.Dir(dropIn), 0o755); err != nil {
		return fmt.Errorf("create drop-in dir: %w", err)
	}
	if err := os.WriteFile(dropIn, []byte("[Service]\nUser=root\n"), 0o644); err != nil {
		return fmt.Errorf("write drop-in: %w", err)
	}
	return reloadAndRestart(systemctl)
}

func demote(dataDir, systemctl string) error {
	dropIn := dropInFile()
	if err := os.Remove(dropIn); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove drop-in: %w", err)
	}
	if err := writeMarker(dataDir, supervisorFile, "systemd"); err != nil {
		return err
	}
	return reloadAndRestart(systemctl)
}

func reloadAndRestart(systemctl string) error {
	unit := unitName()
	if err := runAbs(systemctl, "daemon-reload"); err != nil {
		return err
	}
	return runAbs(systemctl, "--no-block", "restart", unit)
}

func runAbs(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
		}
		return fmt.Errorf("%s %s: %s", bin, strings.Join(args, " "), msg)
	}
	return nil
}

func resolveDataDir() string {
	if v := strings.TrimSpace(os.Getenv("DATA_DIR")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("CC_RUNTIME_DIR")); v != "" {
		agent := filepath.Join(v, "agent")
		if st, err := os.Stat(agent); err == nil && st.IsDir() {
			return agent
		}
		return v
	}
	return defaultDataDir
}

func systemctlBin() string {
	if p := os.Getenv("CC_PRIVCTL_SYSTEMCTL"); p != "" {
		return p
	}
	for _, p := range []string{"/usr/bin/systemctl", "/bin/systemctl"} {
		if isExecutable(p) {
			return p
		}
	}
	return ""
}

func unitName() string {
	if v := strings.TrimSpace(os.Getenv("CC_PRIVCTL_UNIT")); v != "" {
		return v
	}
	return defaultUnit
}

func dropInFile() string {
	if d := strings.TrimSpace(os.Getenv("CC_PRIVCTL_DROPIN_DIR")); d != "" {
		return filepath.Join(d, dropInName)
	}
	return filepath.Join("/etc/systemd/system", unitName()+".d", dropInName)
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func markerPath(dataDir, name string) string {
	return filepath.Join(dataDir, ".run", name)
}

func readMarker(dataDir, name string) string {
	b, err := os.ReadFile(markerPath(dataDir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeMarker(dataDir, name, value string) error {
	path := markerPath(dataDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func ensureServiceUser(dataDir string) error {
	existing := readMarker(dataDir, serviceUserFile)
	if existing != "" && existing != "root" {
		return nil
	}
	user := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if user == "" || user == "root" {
		user = strings.TrimSpace(os.Getenv("USER"))
	}
	if user == "" || user == "root" {
		return fmt.Errorf("cannot determine service user to restore on demote")
	}
	return writeMarker(dataDir, serviceUserFile, user)
}
