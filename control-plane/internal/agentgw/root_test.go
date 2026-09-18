package agentgw

import (
	"errors"
	"strings"
	"testing"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Production change that would fail this test: SendAgentRootCommand not pushing
// AgentRootCommand on the registry, or swallowing ErrAgentOffline when disconnected.
func TestSendAgentRootCommand(t *testing.T) {
	cases := []struct {
		name    string
		online  bool
		enabled bool
		wantErr error
	}{
		{name: "elevate online", online: true, enabled: true},
		{name: "demote online", online: true, enabled: false},
		{name: "offline", online: false, enabled: true, wantErr: ErrAgentOffline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &Gateway{registry: NewRegistry()}
			var conn *Conn
			if tc.online {
				conn = g.registry.Add("srv-1")
			}
			err := g.SendAgentRootCommand("srv-1", tc.enabled)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err=%v want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case msg := <-conn.out:
				cmd := msg.GetAgentRootCommand()
				if cmd == nil {
					t.Fatalf("body=%T want AgentRootCommand", msg.GetBody())
				}
				if cmd.GetEnabled() != tc.enabled {
					t.Fatalf("enabled=%v want %v", cmd.GetEnabled(), tc.enabled)
				}
			default:
				t.Fatal("expected AgentRootCommand on the connection")
			}
		})
	}
}

func TestSendAgentRootCommandClearsStickyRootError(t *testing.T) {
	tr := NewUpdateProgressTracker()
	tr.Record("srv-1", AgentUpdateProgress{Phase: "failed", Detail: "elevate: sudo: a password is required"})
	g := &Gateway{registry: NewRegistry(), progress: tr}
	g.registry.Add("srv-1")
	if err := g.SendAgentRootCommand("srv-1", true); err != nil {
		t.Fatal(err)
	}
	if got := tr.AgentRootError("srv-1"); got != "" {
		t.Fatalf("retry send must clear sticky error, got %q", got)
	}
}

func TestSendAgentRootCommandOfflineKeepsRootError(t *testing.T) {
	tr := NewUpdateProgressTracker()
	tr.Record("srv-1", AgentUpdateProgress{Phase: "failed", Detail: "elevate: sudo: a password is required"})
	g := &Gateway{registry: NewRegistry(), progress: tr}
	err := g.SendAgentRootCommand("srv-1", true)
	if !errors.Is(err, ErrAgentOffline) {
		t.Fatalf("err=%v want offline", err)
	}
	if got := tr.AgentRootError("srv-1"); !strings.Contains(got, "elevate") {
		t.Fatalf("offline send must keep error, got %q", got)
	}
}

func TestSendAgentRootCommandNilGatewayRegistry(t *testing.T) {
	// Same envelope SendAgentUpdate uses, so a connected agent can switch on the oneof.
	g := &Gateway{registry: NewRegistry()}
	conn := g.registry.Add("srv-1")
	if err := g.SendAgentRootCommand("srv-1", true); err != nil {
		t.Fatal(err)
	}
	msg := <-conn.out
	if _, ok := msg.GetBody().(*agentv1.ServerMessage_AgentRootCommand); !ok {
		t.Fatalf("body=%T", msg.GetBody())
	}
}
