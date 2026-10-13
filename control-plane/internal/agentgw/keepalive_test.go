package agentgw

import "testing"

func TestAgentKeepaliveAllowsClientPings(t *testing.T) {
	if !agentKeepaliveEnforcement.PermitWithoutStream {
		t.Fatal("idle pings must be allowed")
	}
	if agentKeepaliveEnforcement.MinTime <= 0 || agentKeepaliveEnforcement.MinTime >= agentPingInterval {
		t.Fatalf("MinTime %s must be positive and under the agent ping interval %s", agentKeepaliveEnforcement.MinTime, agentPingInterval)
	}
}
