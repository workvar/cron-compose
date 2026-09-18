package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Production change that would fail this test: accepting extra argv after
// elevate/demote, which sudoers cannot pin to two exact command lines.
func TestPrivctlArgs(t *testing.T) {
	if err := validateArgs([]string{"elevate"}); err != nil {
		t.Fatal(err)
	}
	if err := validateArgs([]string{"elevate", "extra"}); err == nil {
		t.Fatal("expected reject")
	}
}

func TestPrivctlArgsDemoteAndUnknown(t *testing.T) {
	if err := validateArgs([]string{"demote"}); err != nil {
		t.Fatal(err)
	}
	if err := validateArgs([]string{"root"}); err == nil {
		t.Fatal("expected reject")
	}
	if err := validateArgs(nil); err == nil {
		t.Fatal("expected reject")
	}
}

func TestElevateWritesDropInAndMarkers(t *testing.T) {
	env := newPrivctlEnv(t)
	putMarker(t, env.dataDir, "agent-supervisor", "systemd")
	if err := run([]string{"elevate"}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(env.dropIn)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, "User=root") {
		t.Fatalf("drop-in missing User=root:\n%s", body)
	}

	if got := getMarker(t, env.dataDir, "agent-service-user"); got == "" {
		t.Fatal("expected agent-service-user marker")
	}
	if got := getMarker(t, env.dataDir, "agent-supervisor"); got != "systemd" {
		t.Fatalf("supervisor=%q", got)
	}
	log := readFile(t, env.systemctlLog)
	if !strings.Contains(log, "daemon-reload") || !strings.Contains(log, "restart "+env.unit) {
		t.Fatalf("systemctl log=%q", log)
	}
}

func TestDemoteRemovesDropIn(t *testing.T) {
	env := newPrivctlEnv(t)
	putMarker(t, env.dataDir, "agent-supervisor", "systemd")
	putMarker(t, env.dataDir, "agent-service-user", "croncompose")
	if err := os.MkdirAll(filepath.Dir(env.dropIn), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(env.dropIn, []byte("[Service]\nUser=root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"demote"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(env.dropIn); !os.IsNotExist(err) {
		t.Fatalf("drop-in still present: %v", err)
	}
	if got := getMarker(t, env.dataDir, "agent-service-user"); got != "croncompose" {
		t.Fatalf("service-user=%q, demote must keep original user", got)
	}
	log := readFile(t, env.systemctlLog)
	if !strings.Contains(log, "daemon-reload") || !strings.Contains(log, "restart "+env.unit) {
		t.Fatalf("systemctl log=%q", log)
	}
}

func TestPm2ReturnsSystemdRequiredError(t *testing.T) {
	env := newPrivctlEnv(t)
	putMarker(t, env.dataDir, "agent-supervisor", "pm2")
	err := run([]string{"elevate"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "systemd required to run agent as root under pm2") {
		t.Fatalf("err=%v", err)
	}
	if _, statErr := os.Stat(env.dropIn); !os.IsNotExist(statErr) {
		t.Fatal("pm2 elevate must not write a systemd drop-in")
	}
}

func TestNoSystemctlReturnsSystemdRequiredError(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("CC_PRIVCTL_SYSTEMCTL", filepath.Join(dir, "missing-systemctl"))
	t.Setenv("CC_PRIVCTL_DROPIN_DIR", filepath.Join(dir, "dropins"))
	t.Setenv("CC_PRIVCTL_UNIT", "croncompose-agent.service")
	putMarker(t, dataDir, "agent-supervisor", "systemd")

	err := run([]string{"elevate"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "systemd required to run agent as root under pm2") {
		t.Fatalf("err=%v", err)
	}
}

type privctlEnv struct {
	dataDir      string
	dropIn       string
	systemctlLog string
	unit         string
}

func newPrivctlEnv(t *testing.T) privctlEnv {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	dropDir := filepath.Join(dir, "croncompose-agent.service.d")
	logPath := filepath.Join(dir, "systemctl.log")
	fake := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CC_PRIVCTL_SYSTEMCTL_LOG\"\nexit 0\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	unit := "croncompose-agent.service"
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("CC_PRIVCTL_SYSTEMCTL", fake)
	t.Setenv("CC_PRIVCTL_DROPIN_DIR", dropDir)
	t.Setenv("CC_PRIVCTL_UNIT", unit)
	t.Setenv("CC_PRIVCTL_SYSTEMCTL_LOG", logPath)
	t.Setenv("SUDO_USER", "croncompose")
	return privctlEnv{dataDir: dataDir, dropIn: filepath.Join(dropDir, "root.conf"), systemctlLog: logPath, unit: unit}
}

func putMarker(t *testing.T, dataDir, name, value string) {
	t.Helper()
	path := filepath.Join(dataDir, ".run", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getMarker(t *testing.T, dataDir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dataDir, ".run", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestElevateUsesRuntimeDirAgent(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	dataDir := filepath.Join(runtimeDir, "agent")
	if err := os.MkdirAll(filepath.Join(dataDir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	dropDir := filepath.Join(dir, "dropins")
	logPath := filepath.Join(dir, "systemctl.log")
	fake := filepath.Join(dir, "systemctl")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CC_PRIVCTL_SYSTEMCTL_LOG\"\nexit 0\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", "")
	t.Setenv("CC_RUNTIME_DIR", runtimeDir)
	t.Setenv("CC_PRIVCTL_SYSTEMCTL", fake)
	t.Setenv("CC_PRIVCTL_DROPIN_DIR", dropDir)
	t.Setenv("CC_PRIVCTL_UNIT", "croncompose-agent.service")
	t.Setenv("CC_PRIVCTL_SYSTEMCTL_LOG", logPath)
	t.Setenv("SUDO_USER", "stackuser")
	putMarker(t, dataDir, "agent-supervisor", "systemd")

	if err := run([]string{"elevate"}); err != nil {
		t.Fatal(err)
	}
	if got := getMarker(t, dataDir, "agent-service-user"); got != "stackuser" {
		t.Fatalf("service-user=%q want stackuser at {CC_RUNTIME_DIR}/agent/.run", got)
	}
}

func TestHelperRejectsShellInvocation(t *testing.T) {
	// Guardrail: production code must exec systemctl as argv, never sh -c.
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "sh -c") || strings.Contains(string(src), "bash -c") {
		t.Fatal("privctl must not invoke a shell")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatal(err)
	}
}
