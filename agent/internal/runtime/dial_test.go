package runtime

import (
	"testing"

	"github.com/croncompose/croncompose/agent/internal/config"
	"github.com/croncompose/croncompose/agent/internal/identity"
)

// Production change that would fail this test: ignoring the mode the control plane
// saved at enrollment, so an agent installed through an edge would try mTLS.
func TestConnModeUsesEnrolledModeUnlessEnvOverrides(t *testing.T) {
	r := &Runtime{ident: identity.Identity{GRPCMode: "edge"}}
	if got := r.connMode(); got != "edge" {
		t.Fatalf("enrolled edge: got %q", got)
	}
	r.cfg = config.Config{GRPCMode: "mtls"}
	if got := r.connMode(); got != "mtls" {
		t.Fatalf("env must win: got %q", got)
	}
	if got := (&Runtime{}).connMode(); got != "mtls" {
		t.Fatalf("default: got %q", got)
	}
}
