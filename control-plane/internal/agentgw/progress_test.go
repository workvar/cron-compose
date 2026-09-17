package agentgw

import "testing"

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
