package deploy

import (
	"log/slog"
	"testing"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func stepCmdFor(app, phase string) *agentv1.DeployCommand {
	return &agentv1.DeployCommand{
		RunId: "r1",
		Apps:  []*agentv1.DeployApp{{Name: app}},
		Flow: []*agentv1.DeployFlowWave{{
			Tasks: []*agentv1.DeployFlowTask{{App: app, Phase: phase}},
		}},
	}
}

func TestSkipStepIsPerApp(t *testing.T) {
	m := NewManager(slog.Default(), func(*agentv1.DeployEvent) {})
	m.skipStep(stepCmdFor("Backend", "run"))
	if !m.takeSkip("r1", "Backend", "run") {
		t.Fatal("backend run should be skipped")
	}
	if m.takeSkip("r1", "Backend", "run") {
		t.Fatal("skip flag should be consumed once")
	}
	if m.takeSkip("r1", "Proxy", "run") || m.takeSkip("r1", "Backend", "install") {
		t.Fatal("skip leaked to another app or phase")
	}
}

func TestStopStepSticksToOneApp(t *testing.T) {
	m := NewManager(slog.Default(), func(*agentv1.DeployEvent) {})
	m.stopStep(stepCmdFor("Backend", "build"))
	if m.stickyHalt("r1", "Backend") != "stopped" {
		t.Fatal("backend should stay stopped")
	}
	if m.stickyHalt("r1", "Proxy") != "" || m.stickyHalt("r1", "Frontend") != "" {
		t.Fatal("stop leaked to another app")
	}
	got := m.stoppedApps("r1")
	if len(got) != 1 || got[0] != "Backend" {
		t.Fatalf("stopped apps = %v", got)
	}
}
