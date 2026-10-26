package deploys

import "testing"

func TestFilterFlowOneApp(t *testing.T) {
	flow := [][]FlowTask{
		{{App: "Proxy", Phase: "install"}, {App: "Backend", Phase: "install"}, {App: "Frontend", Phase: "install"}},
		{{App: "Proxy", Phase: "run"}, {App: "Backend", Phase: "run"}, {App: "Frontend", Phase: "run"}},
	}
	got := filterFlow(flow, allowSet([]string{"Backend"}), "")
	if len(got) != 2 {
		t.Fatalf("waves = %d, want 2", len(got))
	}
	for _, wave := range got {
		if len(wave) != 1 || wave[0].App != "Backend" {
			t.Fatalf("wave = %+v, want only Backend", wave)
		}
	}
}

func TestFilterFlowOnePhase(t *testing.T) {
	flow := [][]FlowTask{
		{{App: "Backend", Phase: "install"}, {App: "Backend", Phase: "build"}},
		{{App: "Backend", Phase: "run"}},
	}
	got := filterFlow(flow, allowSet([]string{"backend"}), "run")
	if len(got) != 1 || len(got[0]) != 1 || got[0][0].Phase != "run" {
		t.Fatalf("got %+v", got)
	}
}

func TestAppAllowedDoesNotMatchSiblings(t *testing.T) {
	allow := allowSet([]string{"Backend"})
	if !appAllowed(allow, "Backend", "backend") {
		t.Fatal("backend should match")
	}
	if appAllowed(allow, "Proxy", "proxy") || appAllowed(allow, "Frontend", "frontend") {
		t.Fatal("siblings must stay out of a one-app restart")
	}
}

func TestNormalizePhase(t *testing.T) {
	got, err := normalizePhase("Pre Start")
	if err != nil || got != "pre_start" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := normalizePhase("pm2"); err == nil {
		t.Fatal("expected unknown phase")
	}
}
