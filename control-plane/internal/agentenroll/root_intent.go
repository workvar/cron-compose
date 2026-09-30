package agentenroll

import "context"

// rootIntent maps the enroll request to the agent_root_enabled value to store.
// ok is false for agents that predate run_as_root, which must not change the flag.
func rootIntent(runAsRoot *bool) (enabled, ok bool) {
	if runAsRoot == nil {
		return false, false
	}
	return *runAsRoot, true
}

// recordRootIntent stores whether the agent was just installed to run as root.
//
// Installing with AGENT_RUN_AS_ROOT=1 never went through the "Agent root access"
// toggle, so the flag stayed false while the agent reported root. The toggle then
// read "Off", waited 60s for a mismatch that never settled, and blamed a failed
// demote. The enrolling process runs as root exactly when the installer was asked
// for a root agent, so its uid is the operator's choice. Re-enrolling as a
// dedicated user clears the flag the same way.
func (h *handler) recordRootIntent(ctx context.Context, serverID string, runAsRoot *bool) error {
	enabled, ok := rootIntent(runAsRoot)
	if !ok {
		return nil
	}
	_, err := h.pool.Exec(ctx, `
		update servers set
			agent_root_changed_at = case when agent_root_enabled is distinct from $2 then now() else agent_root_changed_at end,
			agent_root_changed_by = case when agent_root_enabled is distinct from $2 then null else agent_root_changed_by end,
			agent_root_enabled = $2
		where id = $1
	`, serverID, enabled)
	return err
}
