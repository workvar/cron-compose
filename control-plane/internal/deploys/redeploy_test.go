package deploys

import "testing"

func TestNormalizeRedeployOn(t *testing.T) {
	got := NormalizeRedeployOn(nil)
	if len(got) != 1 || got[0] != "branch" {
		t.Fatalf("default = %v", got)
	}
	got = NormalizeRedeployOn([]string{"tag", "branch", "tag", "nope"})
	if len(got) != 2 || got[0] != "tag" || got[1] != "branch" {
		t.Fatalf("got %v", got)
	}
}

func TestTagFromRef(t *testing.T) {
	if got := TagFromRef("refs/tags/v1.2.3"); got != "v1.2.3" {
		t.Fatalf("got %q", got)
	}
	if got := TagFromRef("refs/heads/main"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestWantsRedeploy(t *testing.T) {
	p := Project{RedeployOn: []string{"tag", "release"}}
	if WantsRedeploy(p, "branch") {
		t.Fatal("branch should be off")
	}
	if !WantsRedeploy(p, "tag") || !WantsRedeploy(p, "release") {
		t.Fatal("tag/release should be on")
	}
	if !WantsRedeploy(Project{}, "branch") {
		t.Fatal("empty defaults to branch")
	}
}
