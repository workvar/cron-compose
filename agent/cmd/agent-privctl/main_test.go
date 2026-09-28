package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	if !strings.Contains(body, "ProtectHome=false") {
		t.Fatalf("drop-in missing ProtectHome=false (packaged unit hardening):\n%s", body)
	}
	if !strings.Contains(body, "UnsetEnvironment=AGENT_VERSION") {
		t.Fatalf("drop-in missing UnsetEnvironment=AGENT_VERSION:\n%s", body)
	}
	if !strings.Contains(body, "ProtectSystem=false") {
		t.Fatalf("drop-in missing ProtectSystem=false:\n%s", body)
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

// Production change that would fail this test: waiting on `systemctl restart`
// of the unit that contains this helper, so systemd SIGTERMs CombinedOutput.
func TestRestartDoesNotBlockOnUnit(t *testing.T) {
	env := newPrivctlEnv(t)
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CC_PRIVCTL_SYSTEMCTL_LOG\"\n" +
		"noblock=0\n" +
		"restart=0\n" +
		"for a in \"$@\"; do\n" +
		"  [ \"$a\" = restart ] && restart=1\n" +
		"  [ \"$a\" = --no-block ] && noblock=1\n" +
		"done\n" +
		"[ \"$1\" = show ] && { echo root; exit 0; }\n" +
		"if [ \"$restart\" = 1 ] && [ \"$noblock\" != 1 ]; then echo 'blocking restart' >&2; exit 99; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(env.systemctlBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	putMarker(t, env.dataDir, "agent-supervisor", "systemd")
	if err := run([]string{"elevate"}); err != nil {
		t.Fatal(err)
	}
	log := readFile(t, env.systemctlLog)
	if !strings.Contains(log, "--no-block") || !strings.Contains(log, "restart") {
		t.Fatalf("expected --no-block restart, got %q", log)
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
	fakeProc(t, dir, "0::/system.slice/croncompose-agent.service")
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
	systemctlBin string
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
	script := fakeSystemctl("root")
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
	fakeProc(t, dir, "0::/system.slice/"+unit)
	return privctlEnv{dataDir: dataDir, dropIn: filepath.Join(dropDir, "root.conf"), systemctlLog: logPath, systemctlBin: fake, unit: unit}
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
	script := fakeSystemctl("root")
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
	fakeProc(t, dir, "0::/system.slice/croncompose-agent.service")
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

// fakeSystemctl logs argv and answers `show -p User --value` with user.
func fakeSystemctl(user string) string {
	return "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CC_PRIVCTL_SYSTEMCTL_LOG\"\n" +
		"[ \"$1\" = show ] && echo '" + user + "'\nexit 0\n"
}

// fakeProc builds <dir>/proc with sudo (the test's parent pid) in a session
// scope and its parent (the "agent", pid 4242) in agentCgroup.
func fakeProc(t *testing.T, dir, agentCgroup string) {
	t.Helper()
	proc := filepath.Join(dir, "proc")
	write := func(pid int, cgroup string, ppid int) {
		d := filepath.Join(proc, strconv.Itoa(pid))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "cgroup"), []byte(cgroup+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		status := "Name:\tx\nPPid:\t" + strconv.Itoa(ppid) + "\n"
		if err := os.WriteFile(filepath.Join(d, "status"), []byte(status), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(os.Getppid(), "0::/user.slice/user-0.slice/session-9.scope", 4242)
	write(4242, agentCgroup, 1)
	t.Setenv("CC_PRIVCTL_PROC", proc)
}

// Production change that would fail this test: dropping the cgroup preflight,
// so an agent under pm2/nohup restarts an unrelated unit and the UI times out
// with no error.
func TestElevateRejectsAgentOutsideUnit(t *testing.T) {
	env := newPrivctlEnv(t)
	putMarker(t, env.dataDir, "agent-supervisor", "systemd")
	fakeProc(t, t.TempDir(), "0::/user.slice/user-1000.slice/user@1000.service/app.slice/pm2.service")
	err := run([]string{"elevate"})
	if err == nil || !strings.Contains(err.Error(), "not running as croncompose-agent.service") {
		t.Fatalf("err=%v", err)
	}
	if _, statErr := os.Stat(env.dropIn); !os.IsNotExist(statErr) {
		t.Fatal("must not write a drop-in when the agent is outside the unit")
	}
}

// Production change that would fail this test: restarting even though another
// drop-in still sets User=, so the agent silently comes back unprivileged.
func TestElevateRejectsOverriddenUser(t *testing.T) {
	env := newPrivctlEnv(t)
	putMarker(t, env.dataDir, "agent-supervisor", "systemd")
	if err := os.WriteFile(env.systemctlBin, []byte(fakeSystemctl("croncompose")), 0o755); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"elevate"})
	if err == nil || !strings.Contains(err.Error(), `User="croncompose"`) {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(readFile(t, env.systemctlLog), "restart") {
		t.Fatal("must not restart when User= is overridden")
	}
}

func TestCgroupHasUnit(t *testing.T) {
	unit := "croncompose-agent.service"
	cases := map[string]bool{
		"0::/system.slice/croncompose-agent.service\n":                      true,
		"12:pids:/system.slice/croncompose-agent.service\n0::/init.scope\n": true,
		"0::/system.slice/croncompose-agent.service.d\n":                    false,
		"0::/user.slice/user-1000.slice/session-3.scope\n":                  false,
		"": false,
	}
	for cg, want := range cases {
		if got := cgroupHasUnit(cg, unit); got != want {
			t.Errorf("cgroupHasUnit(%q)=%v want %v", cg, got, want)
		}
	}
}
