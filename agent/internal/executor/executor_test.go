//go:build !windows

package executor

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestRunSucceedsAndCapturesStdout(t *testing.T) {
	var mu sync.Mutex
	var out []string
	sink := func(stream string, seq int, data []byte) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, stream+":"+string(data))
	}
	res := Run(context.Background(), Job{
		Interpreter:    "bash",
		ScriptBody:     `echo hello`,
		TimeoutSeconds: 5,
	}, sink)
	if res.Status != "succeeded" {
		t.Fatalf("status: %q (err=%q)", res.Status, res.Err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code: %d", res.ExitCode)
	}
	joined := strings.Join(out, "|")
	if !strings.Contains(joined, "stdout:hello") {
		t.Errorf("did not see stdout chunk: %q", joined)
	}
}

func TestRunPropagatesNonZeroExit(t *testing.T) {
	res := Run(context.Background(), Job{
		Interpreter:    "bash",
		ScriptBody:     `exit 7`,
		TimeoutSeconds: 5,
	}, func(string, int, []byte) {})
	if res.Status != "failed" {
		t.Errorf("status: got %q, want failed", res.Status)
	}
	if res.ExitCode != 7 {
		t.Errorf("exit code: got %d, want 7", res.ExitCode)
	}
}

func TestRunHonorsTimeout(t *testing.T) {
	res := Run(context.Background(), Job{
		Interpreter:    "bash",
		ScriptBody:     `sleep 5`,
		TimeoutSeconds: 1,
	}, func(string, int, []byte) {})
	if res.Status != "timed_out" {
		t.Errorf("status: got %q, want timed_out", res.Status)
	}
}
