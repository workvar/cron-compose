package servers

import (
	"strings"
	"testing"
)

func newCommandHandler() *handler {
	return &handler{endpoints: Endpoints{
		InstallScriptURL: "https://example.test/install-agent.sh",
		PublicHTTPURL:    "https://cc.example.test/api",
		PublicGRPCAddr:   "cc.example.test:9077",
	}}
}

// Production change that would fail this test: emitting the old CONTROL_PLANE_*
// names again, which collide with the control plane's own settings on a shared host.
func TestInstallCommandUsesAgentVariableNames(t *testing.T) {
	cmd := newCommandHandler().installCommand("tok", false)
	for _, want := range []string{"TOKEN=tok", "AGENT_ENROLL_HTTP=https://cc.example.test/api", "AGENT_GRPC_ADDR=cc.example.test:9077"} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("missing %q in %q", want, cmd)
		}
	}
	if strings.Contains(cmd, "CONTROL_PLANE_") || strings.Contains(cmd, "AGENT_RUN_AS_ROOT") {
		t.Fatalf("unexpected content in %q", cmd)
	}
}

// The variables must sit after sudo so they reach the installer, not curl.
func TestInstallCommandRootSetsRunAsRootAfterSudo(t *testing.T) {
	cmd := newCommandHandler().installCommand("tok", true)
	sudo := strings.Index(cmd, "| sudo ")
	if sudo < 0 || strings.Index(cmd, "AGENT_RUN_AS_ROOT=1") < sudo || strings.Index(cmd, "AGENT_GRPC_ADDR=") < sudo {
		t.Fatalf("variables must follow sudo: %q", cmd)
	}
	if !strings.HasSuffix(cmd, " bash") {
		t.Fatalf("got %q", cmd)
	}
}
