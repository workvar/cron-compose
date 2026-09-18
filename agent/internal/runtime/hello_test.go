package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/croncompose/croncompose/agent/internal/config"
)

// Production change that would fail this test: ignoring .run/agent-service-user
// and always reporting USER, so demote loses the original service account.
func TestServiceUserPrefersMarker(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".run", "agent-service-user"), []byte("croncompose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USER", "other")
	if got := serviceUser(dir); got != "croncompose" {
		t.Fatalf("serviceUser=%q want croncompose", got)
	}
}

// Production change that would fail this test: requiring a marker file and
// reporting empty when USER is set (Hello would persist a blank service_user).
func TestServiceUserFallsBackToUSER(t *testing.T) {
	t.Setenv("USER", "pi")
	if got := serviceUser(t.TempDir()); got != "pi" {
		t.Fatalf("serviceUser=%q want pi", got)
	}
}

func TestNewHelloReportsEuidAndServiceUser(t *testing.T) {
	t.Setenv("USER", "agentuser")
	r := &Runtime{cfg: config.Config{DataDir: t.TempDir(), AgentVersion: "test"}}
	h := r.newHello()
	if h.GetServiceUser() != "agentuser" {
		t.Fatalf("service_user=%q", h.GetServiceUser())
	}
	wantRoot := os.Geteuid() == 0
	if h.GetEuidRoot() != wantRoot {
		t.Fatalf("euid_root=%v want %v", h.GetEuidRoot(), wantRoot)
	}
	if h.GetAgentVersion() != "test" {
		t.Fatalf("agent_version=%q", h.GetAgentVersion())
	}
}
