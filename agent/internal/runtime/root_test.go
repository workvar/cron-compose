package runtime

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/croncompose/croncompose/agent/internal/config"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func TestHandleAgentRootElevateInvokesHelper(t *testing.T) {
	logPath := writePrivctlFakes(t, 0, "")
	r := newRootTestRuntime(t)
	r.handleAgentRootCommand(true)
	got := readTestFile(t, logPath)
	if !strings.Contains(got, "-n ") || !strings.Contains(got, " elevate") {
		t.Fatalf("expected sudo -n … elevate, got %q", got)
	}
	if strings.Contains(got, "demote") {
		t.Fatalf("elevate invoked demote: %q", got)
	}
}

func TestHandleAgentRootDisableRunsDemote(t *testing.T) {
	logPath := writePrivctlFakes(t, 0, "")
	r := newRootTestRuntime(t)
	r.handleAgentRootCommand(false)
	got := readTestFile(t, logPath)
	if !strings.Contains(got, " demote") {
		t.Fatalf("expected demote, got %q", got)
	}
}

func TestHandleAgentRootFailureSendsEphemeralError(t *testing.T) {
	writePrivctlFakes(t, 1, "sudo: a password is required")
	r := newRootTestRuntime(t)
	r.handleAgentRootCommand(true)
	select {
	case msg := <-r.direct:
		detail := msg.GetUpdateProgress().GetDetail()
		if !strings.Contains(detail, "password is required") && !strings.Contains(detail, "elevate") {
			t.Fatalf("ephemeral error %q", detail)
		}
		if msg.GetUpdateProgress().GetPhase() != "failed" {
			t.Fatalf("phase=%q", msg.GetUpdateProgress().GetPhase())
		}
	default:
		t.Fatal("expected ephemeral error on direct-send")
	}
}

func TestHandleServerMessageAgentRootDoesNotPanic(t *testing.T) {
	writePrivctlFakes(t, 0, "")
	r := newRootTestRuntime(t)
	r.handleServerMessage(context.Background(), &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_AgentRootCommand{
			AgentRootCommand: &agentv1.AgentRootCommand{Enabled: false},
		},
	})
}

func newRootTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	return &Runtime{
		cfg:    config.Config{DataDir: t.TempDir()},
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		direct: make(chan *agentv1.AgentMessage, 8),
	}
}

func writePrivctlFakes(t *testing.T, exitCode int, stderr string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "invoke.log")
	helper := filepath.Join(dir, "agent-privctl")
	sudo := filepath.Join(dir, "sudo")

	helperScript := "#!/bin/sh\nprintf 'helper %s\\n' \"$*\" >> \"$CC_PRIVCTL_TEST_LOG\"\n"
	if stderr != "" {
		helperScript += "printf '%s\\n' '" + stderr + "' >&2\n"
	}
	helperScript += "exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(helper, []byte(helperScript), 0o755); err != nil {
		t.Fatal(err)
	}

	sudoScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CC_PRIVCTL_TEST_LOG\"\nshift\nshift\nexec \"$CC_AGENT_PRIVCTL\" \"$@\"\n"
	if err := os.WriteFile(sudo, []byte(sudoScript), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CC_PRIVCTL_TEST_LOG", logPath)
	t.Setenv("CC_AGENT_PRIVCTL", helper)
	t.Setenv("CC_AGENT_PRIVCTL_SUDO", sudo)
	return logPath
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
