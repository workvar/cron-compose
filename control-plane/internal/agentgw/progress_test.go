package agentgw

import (
	"strings"
	"testing"
)

func TestUpdateProgressTrackerOfferAndRecord(t *testing.T) {
	tr := NewUpdateProgressTracker()
	tr.Offer("srv1", "v1.2.0")

	got := tr.Snapshot("srv1", "v1.1.0")
	if got == nil {
		t.Fatal("expected offered progress")
	}
	if got.Phase != "offered" || got.Percent != 5 || got.TargetVersion != "v1.2.0" {
		t.Fatalf("offer snapshot = %+v", got)
	}
	if got.Detail == "" {
		t.Fatal("offer should include a human-readable detail")
	}

	tr.Record("srv1", AgentUpdateProgress{
		TargetVersion: "v1.2.0",
		Phase:         "downloading",
		Detail:        "Downloading and building Docker images",
		Percent:       40,
	})
	got = tr.Snapshot("srv1", "v1.1.0")
	if got == nil || got.Phase != "downloading" || got.Percent != 40 {
		t.Fatalf("recorded snapshot = %+v", got)
	}
}

func TestUpdateProgressTrackerDoneWhenVersionMatches(t *testing.T) {
	tr := NewUpdateProgressTracker()
	tr.Offer("srv1", "v1.2.0")
	tr.Record("srv1", AgentUpdateProgress{
		TargetVersion: "v1.2.0",
		Phase:         "restarting",
		Detail:        "Starting updated containers",
		Percent:       88,
	})

	got := tr.Snapshot("srv1", "v1.2.0")
	if got == nil || got.Phase != "done" || got.Percent != 100 {
		t.Fatalf("matched version should report done, got %+v", got)
	}
}

func TestUpdateProgressTrackerUnknownServer(t *testing.T) {
	tr := NewUpdateProgressTracker()
	if tr.Snapshot("missing", "v1") != nil {
		t.Fatal("missing server should have no progress")
	}
}

func TestUpdateProgressTrackerClear(t *testing.T) {
	tr := NewUpdateProgressTracker()
	tr.Offer("srv1", "v2")
	tr.Clear("srv1")
	if tr.Snapshot("srv1", "v1") != nil {
		t.Fatal("cleared progress should be gone")
	}
}

// Production change that would fail this test: recording privctl UpdateProgress
// {phase:failed, target empty} so GET /updates shows an agent-root failure.
func TestUpdateProgressTrackerIgnoresAgentRootFailure(t *testing.T) {
	tr := NewUpdateProgressTracker()
	tr.Offer("srv1", "v1.2.0")
	tr.Record("srv1", AgentUpdateProgress{
		Phase:  "failed",
		Detail: "elevate: sudo: a password is required; grant: ALL=(root) NOPASSWD: /usr/libexec/croncompose/agent-privctl elevate",
	})
	got := tr.Snapshot("srv1", "v1.1.0")
	if got == nil || got.Phase != "offered" || got.TargetVersion != "v1.2.0" {
		t.Fatalf("agent-root failure must not replace self-update progress, got %+v", got)
	}

	tr2 := NewUpdateProgressTracker()
	tr2.Record("srv2", AgentUpdateProgress{
		Phase:  "failed",
		Detail: "demote: systemd required to run agent as root under pm2",
	})
	if tr2.Snapshot("srv2", "v1") != nil {
		t.Fatal("agent-root failure must not appear as update progress")
	}
	if got := tr2.AgentRootError("srv2"); !strings.Contains(got, "systemd required") {
		t.Fatalf("AgentRootError=%q", got)
	}
}
