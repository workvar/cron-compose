package agentgw

import (
	"errors"
	"log/slog"
	"testing"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func newRepushService() *service {
	return &service{
		log:      slog.Default(),
		registry: NewRegistry(),
		progress: NewUpdateProgressTracker(),
		rootSent: newRootDelivery(),
	}
}

// drain returns the AgentRootCommand queued on conn, or nil when nothing is queued.
func drain(t *testing.T, conn *Conn) *agentv1.AgentRootCommand {
	t.Helper()
	select {
	case msg := <-conn.out:
		cmd := msg.GetAgentRootCommand()
		if cmd == nil {
			t.Fatalf("body=%T want AgentRootCommand", msg.GetBody())
		}
		return cmd
	default:
		return nil
	}
}

// Production change that would fail this test: resendNeeded ignoring what was already
// delivered (restart loop on a host that cannot elevate), or never re-sending a value
// that was not delivered (the original stuck toggle).
func TestRootDeliveryResendNeeded(t *testing.T) {
	cases := []struct {
		name      string
		sentValue *bool
		enabled   bool
		euidRoot  bool
		want      bool
	}{
		{name: "agree, nothing sent", enabled: true, euidRoot: true, want: false},
		{name: "wants root, not root, never sent", enabled: true, euidRoot: false, want: true},
		{name: "wants demote, still root, never sent", enabled: false, euidRoot: true, want: true},
		{name: "wants root, already delivered", sentValue: ptr(true), enabled: true, euidRoot: false, want: false},
		{name: "wants demote, only elevate was delivered", sentValue: ptr(true), enabled: false, euidRoot: true, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newRootDelivery()
			if tc.sentValue != nil {
				d.markSent("srv-1", *tc.sentValue)
			}
			if got := d.resendNeeded("srv-1", tc.enabled, tc.euidRoot); got != tc.want {
				t.Fatalf("resendNeeded=%v want %v", got, tc.want)
			}
		})
	}
}

func ptr(b bool) *bool { return &b }

// Production change that would fail this test: keeping the delivered mark after Hello
// agrees with the flag, so later drift is never corrected.
func TestRootDeliverySettledClearsMark(t *testing.T) {
	d := newRootDelivery()
	d.markSent("srv-1", true)
	if d.resendNeeded("srv-1", true, true) {
		t.Fatal("settled state must not resend")
	}
	if !d.resendNeeded("srv-1", true, false) {
		t.Fatal("drift after settling must be corrected once")
	}
}

func TestRootDeliveryNilReceiverIsSafe(t *testing.T) {
	var d *rootDelivery
	d.markSent("srv-1", true)
	if d.resendNeeded("srv-1", true, false) {
		t.Fatal("nil receiver must never resend")
	}
}

// The reported bug: the operator toggles root while the agent is offline, the flag is
// stored but the command is dropped, and nothing re-sends it on reconnect.
// Production change that would fail this test: repushRootCommand not delivering a command
// that was dropped while the agent was offline (onHello wiring is covered by the DB test).
func TestRepushDeliversCommandDroppedWhileOffline(t *testing.T) {
	g := &Gateway{registry: NewRegistry(), progress: NewUpdateProgressTracker(), rootSent: newRootDelivery()}
	if err := g.SendAgentRootCommand("srv-1", true); !errors.Is(err, ErrAgentOffline) {
		t.Fatalf("err=%v want offline", err)
	}

	s := &service{log: slog.Default(), registry: g.registry, progress: g.progress, rootSent: g.rootSent}
	conn := s.registry.Add("srv-1") // the agent reconnects
	s.repushRootCommand("srv-1", true, false)

	cmd := drain(t, conn)
	if cmd == nil || !cmd.GetEnabled() {
		t.Fatalf("expected an elevate command after reconnect, got %v", cmd)
	}
}

// Production change that would fail this test: re-sending on every Hello, which
// restarts the agent forever when it cannot elevate.
func TestRepushSendsAtMostOncePerDesiredValue(t *testing.T) {
	s := newRepushService()
	conn := s.registry.Add("srv-1")

	s.repushRootCommand("srv-1", true, false)
	if drain(t, conn) == nil {
		t.Fatal("first mismatch must re-send")
	}
	s.repushRootCommand("srv-1", true, false)
	if drain(t, conn) != nil {
		t.Fatal("second identical mismatch must not re-send")
	}
	s.repushRootCommand("srv-1", false, true) // operator flips the desired value
	if cmd := drain(t, conn); cmd == nil || cmd.GetEnabled() {
		t.Fatalf("a new desired value must be sent once, got %v", cmd)
	}
}

// Production change that would fail this test: the operator toggle not recording its
// delivery, so the next Hello sends the same command a second time.
func TestRepushSkipsCommandTheToggleAlreadyDelivered(t *testing.T) {
	g := &Gateway{registry: NewRegistry(), progress: NewUpdateProgressTracker(), rootSent: newRootDelivery()}
	conn := g.registry.Add("srv-1")
	if err := g.SendAgentRootCommand("srv-1", true); err != nil {
		t.Fatal(err)
	}
	if drain(t, conn) == nil {
		t.Fatal("toggle must deliver the command")
	}

	s := &service{log: slog.Default(), registry: g.registry, progress: g.progress, rootSent: g.rootSent}
	s.repushRootCommand("srv-1", true, false) // agent restarts, still not root yet
	if drain(t, conn) != nil {
		t.Fatal("already delivered command must not be duplicated")
	}
}

func TestRepushDoesNothingWhenAgentAgrees(t *testing.T) {
	s := newRepushService()
	conn := s.registry.Add("srv-1")
	s.repushRootCommand("srv-1", true, true)
	s.repushRootCommand("srv-1", false, false)
	if drain(t, conn) != nil {
		t.Fatal("agreeing state must not send anything")
	}
}

// A failed send must not count as delivered, or the next reconnect would skip it.
func TestRepushOfflineSendIsNotMarkedDelivered(t *testing.T) {
	s := newRepushService()
	s.repushRootCommand("srv-1", true, false) // no connection registered
	conn := s.registry.Add("srv-1")
	s.repushRootCommand("srv-1", true, false)
	if cmd := drain(t, conn); cmd == nil || !cmd.GetEnabled() {
		t.Fatalf("expected the command once the agent is reachable, got %v", cmd)
	}
}

// Production change that would fail this test: a re-send leaving the previous
// privctl error on screen, so the UI shows a stale failure for a fresh attempt.
func TestRepushClearsStickyRootError(t *testing.T) {
	s := newRepushService()
	s.progress.Record("srv-1", AgentUpdateProgress{Phase: "failed", Detail: "elevate: sudo: a password is required"})
	s.registry.Add("srv-1")
	s.repushRootCommand("srv-1", true, false)
	if got := s.progress.AgentRootError("srv-1"); got != "" {
		t.Fatalf("sticky error must clear on re-send, got %q", got)
	}
}
