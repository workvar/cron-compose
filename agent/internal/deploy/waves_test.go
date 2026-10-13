package deploy

import (
	"testing"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func TestGroupAppsByStart(t *testing.T) {
	apps := []*agentv1.DeployApp{
		{Name: "Frontend", StartGroup: 2},
		{Name: "Proxy", StartGroup: 0},
		{Name: "Backend", StartGroup: 0},
		{Name: "Worker", StartGroup: 1},
	}
	waves := groupAppsByStart(apps)
	if len(waves) != 3 {
		t.Fatalf("waves=%d want 3", len(waves))
	}
	if got := names(waves[0]); got != "Proxy,Backend" {
		t.Fatalf("wave0=%q", got)
	}
	if got := names(waves[1]); got != "Worker" {
		t.Fatalf("wave1=%q", got)
	}
	if got := names(waves[2]); got != "Frontend" {
		t.Fatalf("wave2=%q", got)
	}
}

func TestGroupAppsByStartDefault(t *testing.T) {
	apps := []*agentv1.DeployApp{{Name: "a"}, {Name: "b"}}
	waves := groupAppsByStart(apps)
	if len(waves) != 1 || len(waves[0]) != 2 {
		t.Fatalf("expected one wave with both apps, got %#v", waves)
	}
}

func names(apps []*agentv1.DeployApp) string {
	out := ""
	for i, a := range apps {
		if i > 0 {
			out += ","
		}
		out += a.GetName()
	}
	return out
}
