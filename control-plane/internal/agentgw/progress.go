package agentgw

import (
	"strings"
	"sync"
	"time"
)

// AgentUpdateProgress is the latest self-update stage reported by one agent.
type AgentUpdateProgress struct {
	TargetVersion string
	Phase         string
	Detail        string
	Percent       int
	UpdatedAt     time.Time
}

// UpdateProgressTracker holds in-memory update stages keyed by server ID. The
// UI polls GET /updates; there is no reason to persist this across control-plane
// restarts (a restart itself is a stage of a stack update).
type UpdateProgressTracker struct {
	mu       sync.Mutex
	byServer map[string]AgentUpdateProgress
	rootErr  map[string]string
}

func NewUpdateProgressTracker() *UpdateProgressTracker {
	return &UpdateProgressTracker{
		byServer: map[string]AgentUpdateProgress{},
		rootErr:  map[string]string{},
	}
}

// Offer records that the control plane just pushed UpdateAgent to this server.
func (t *UpdateProgressTracker) Offer(serverID, targetVersion string) {
	if t == nil {
		return
	}
	t.Record(serverID, AgentUpdateProgress{
		TargetVersion: targetVersion,
		Phase:         "offered",
		Detail:        "Sending the update command to the agent",
		Percent:       5,
	})
}

func (t *UpdateProgressTracker) Record(serverID string, p AgentUpdateProgress) {
	if t == nil || serverID == "" {
		return
	}
	// Privctl reuses UpdateProgress with phase=failed and no target version.
	// Drop those so GET /updates never treats an agent-root failure as a self-update.
	if strings.TrimSpace(p.TargetVersion) == "" {
		if p.Phase == "failed" && strings.TrimSpace(p.Detail) != "" {
			t.mu.Lock()
			if t.rootErr == nil {
				t.rootErr = map[string]string{}
			}
			t.rootErr[serverID] = p.Detail
			t.mu.Unlock()
		}
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if p.Percent == 0 {
		if prev, ok := t.byServer[serverID]; ok {
			p.Percent = prev.Percent
		}
	}
	p.UpdatedAt = time.Now()
	t.byServer[serverID] = p
}

// Snapshot returns the live stage for a server. When the agent has already
// come back on the target version, the snapshot is forced to "done" so the UI
// can close the overlay even if the last event was "restarting".
func (t *UpdateProgressTracker) Snapshot(serverID, currentVersion string) *AgentUpdateProgress {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.byServer[serverID]
	if !ok {
		return nil
	}
	out := p
	if out.Phase != "failed" && VersionsEqual(currentVersion, out.TargetVersion) {
		out.Phase = "done"
		out.Detail = "Update complete"
		out.Percent = 100
	}
	return &out
}

// AgentRootError is the last privctl failure for this server, if any. Empty
// when the agent has not reported one. Never mixed into Snapshot / GET /updates.
func (t *UpdateProgressTracker) AgentRootError(serverID string) string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rootErr[serverID]
}

func (t *UpdateProgressTracker) ClearRootError(serverID string) {
	if t == nil || serverID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.rootErr, serverID)
}

// ReconcileRootError drops a sticky privctl error once Hello reports euid
// matching the desired flag (enabled && root, or disabled && not root).
func (t *UpdateProgressTracker) ReconcileRootError(serverID string, enabled, euidRoot bool) {
	if enabled == euidRoot {
		t.ClearRootError(serverID)
	}
}

func (t *UpdateProgressTracker) Clear(serverID string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byServer, serverID)
}
