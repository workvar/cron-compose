package agentgw

import (
	"time"
)

// offlineGrace is how long a server stays "online" in the DB after its AgentStream
// drops. Brief reconnects (proxy idle cuts, agent backoff) used to flip the UI to
// offline between heartbeats and look like an outage. Commands still need a live
// registry Conn; only the displayed status is deferred.
var offlineGrace = 90 * time.Second

// scheduleOffline marks the server offline after offlineGrace unless it reconnects.
// Repeated disconnects reset the timer so a flapping link does not clear the grace.
func (s *service) scheduleOffline(serverID string) {
	s.offlineMu.Lock()
	defer s.offlineMu.Unlock()
	if s.offlineTimers == nil {
		s.offlineTimers = map[string]*time.Timer{}
	}
	if t, ok := s.offlineTimers[serverID]; ok {
		t.Stop()
	}
	s.offlineTimers[serverID] = time.AfterFunc(offlineGrace, func() {
		s.offlineMu.Lock()
		delete(s.offlineTimers, serverID)
		s.offlineMu.Unlock()
		if s.registry.Connected(serverID) {
			return
		}
		s.markOffline(serverID)
	})
}

// cancelPendingOffline drops a deferred offline mark when the agent is back.
func (s *service) cancelPendingOffline(serverID string) {
	s.offlineMu.Lock()
	defer s.offlineMu.Unlock()
	if t, ok := s.offlineTimers[serverID]; ok {
		t.Stop()
		delete(s.offlineTimers, serverID)
	}
}
