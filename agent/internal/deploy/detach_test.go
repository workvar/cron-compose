package deploy

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// A dropped agent stream used to cancel the install, because Handle threaded the
// stream context into the run. The run must keep going; only CloseAll (process
// shutdown) and an explicit cancel stop it early.
func TestHandleIgnoresCanceledStreamContext(t *testing.T) {
	finished := make(chan string, 1)
	m := NewManager(slog.New(slog.DiscardHandler), func(ev *agentv1.DeployEvent) {
		if ev.GetKind() == "finished" {
			finished <- ev.GetMessage()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m.Handle(ctx, &agentv1.DeployCommand{
		RunId:          "run-detach",
		DestPath:       t.TempDir(),
		Branch:         "main",
		CloneUrl:       "file:///tmp/croncompose-no-such-repo",
		InstallScript:  "true",
		TimeoutSeconds: 20,
	})

	select {
	case msg := <-finished:
		if strings.Contains(msg, "context canceled") {
			t.Fatalf("stream cancel aborted the deploy: %s", msg)
		}
	case <-time.After(25 * time.Second):
		t.Fatal("deploy did not finish")
	}
}

func TestCloseAllCancelsInstall(t *testing.T) {
	m := NewManager(slog.New(slog.DiscardHandler), func(*agentv1.DeployEvent) {})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	m.mu.Lock()
	m.cancel["run"] = cancel
	m.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.runPTY(ctx, "run", "", t.TempDir(), "sleep 30", nil, nil, "", false)
	}()
	time.Sleep(300 * time.Millisecond)
	m.CloseAll()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("install was not canceled")
	}
}
