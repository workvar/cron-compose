package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// agentRootMessage builds the AgentRootCommand pushed to an agent. It is shared by
// the operator toggle (Gateway.SendAgentRootCommand) and the Hello re-push below.
func agentRootMessage(enabled bool) *agentv1.ServerMessage {
	return &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_AgentRootCommand{
			AgentRootCommand: &agentv1.AgentRootCommand{Enabled: enabled},
		},
	}
}

// rootDelivery remembers, per server, the desired agent-root value whose command
// was last delivered to the agent's stream.
//
// The desired flag is stored before the command is pushed, so a command sent while
// the agent was offline or reconnecting is lost and the flag never settles. A Hello
// that still disagrees with the flag therefore re-sends it, but only once per
// desired value: if the host cannot elevate (privctl fails, or the agent runs under
// pm2), sending again on every reconnect would only restart the agent in a loop.
// The entry is cleared once Hello agrees with the flag, so later drift is corrected
// once more.
//
// All methods are safe on a nil receiver, so a Gateway built without one (as in
// unit tests) simply never re-sends.
type rootDelivery struct {
	mu   sync.Mutex
	sent map[string]bool
}

func newRootDelivery() *rootDelivery {
	return &rootDelivery{sent: map[string]bool{}}
}

// markSent records that the command for enabled reached the agent's stream.
func (d *rootDelivery) markSent(serverID string, enabled bool) {
	if d == nil || serverID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sent[serverID] = enabled
}

// resendNeeded reports whether Hello should push the desired value again: the agent
// disagrees with the stored flag and that value has not been delivered yet.
func (d *rootDelivery) resendNeeded(serverID string, enabled, euidRoot bool) bool {
	if d == nil || serverID == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if enabled == euidRoot {
		delete(d.sent, serverID)
		return false
	}
	sent, ok := d.sent[serverID]
	return !ok || sent != enabled
}

// repushRootCommand re-sends the desired agent-root command after a Hello that
// disagrees with the stored flag. A false flag that no operator set (legacy root
// installs, macOS root agents, which predate the flag) is never turned into a
// demote, since it cannot be told apart from a flag nobody chose. Failures are logged; the operator can still retry
// from the toggle.
func (s *service) repushRootCommand(serverID string, enabled, euidRoot, operatorSet bool) {
	if !s.rootSent.resendNeeded(serverID, enabled, euidRoot) {
		return
	}
	if !enabled && !operatorSet {
		return
	}
	if err := s.registry.Send(serverID, agentRootMessage(enabled)); err != nil {
		s.log.Warn("re-send agent root command failed", "server_id", serverID, "enabled", enabled, "err", err)
		return
	}
	s.rootSent.markSent(serverID, enabled)
	s.progress.ClearRootError(serverID)
	s.log.Info("re-sent agent root command after hello", "server_id", serverID, "enabled", enabled, "euid_root", euidRoot)
}
