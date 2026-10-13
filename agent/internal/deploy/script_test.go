package deploy

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Installers that check stdin.isTTY (pnpm approve-builds) must not see a TTY,
// or the deploy hangs waiting for an interactive picker.
func TestRunScriptStdinIsNotTTY(t *testing.T) {
	var logs bytes.Buffer
	m := NewManager(slog.New(slog.DiscardHandler), func(ev *agentv1.DeployEvent) {
		if ev.GetKind() == "log" {
			logs.Write(ev.GetData())
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := m.runScript(ctx, "run-tty", "", t.TempDir(),
		`if [ -t 0 ]; then echo STDIN_IS_TTY; else echo STDIN_NOT_TTY; fi`,
		nil, nil, "", false)
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	out := logs.String()
	if !strings.Contains(out, "STDIN_NOT_TTY") {
		t.Fatalf("expected non-TTY stdin, got %q", out)
	}
	if strings.Contains(out, "STDIN_IS_TTY") {
		t.Fatalf("stdin looked like a TTY: %q", out)
	}
}

func TestRunScriptTagLogsPrefix(t *testing.T) {
	var logs bytes.Buffer
	m := NewManager(slog.New(slog.DiscardHandler), func(ev *agentv1.DeployEvent) {
		if ev.GetKind() == "log" {
			logs.Write(ev.GetData())
		}
	})
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := m.runScript(ctx, "run-tag", "", dir, `echo hello-from-app`, nil, nil, "", true)
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	out := logs.String()
	want := "[in " + dir + "] hello-from-app"
	if !strings.Contains(out, want) {
		t.Fatalf("missing tagged line %q in %q", want, out)
	}
}
