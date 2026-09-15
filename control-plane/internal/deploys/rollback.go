package deploys

import (
	"context"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// triggerRollback marks the runs this package starts itself. It is never accepted
// from a caller: a run already marked as a rollback does not trigger another one, so
// being able to set it from outside would be a way to switch auto-rollback off.
const triggerRollback = "rollback"

// healthCheckFor renders the project's opt-in probe for the agent. A project with no
// health path gets nil, and the agent treats the install script's exit code as the
// whole verdict, exactly as it did before health checks existed.
func healthCheckFor(p Project) *agentv1.HealthCheck {
	if p.HealthPath == "" {
		return nil
	}
	return &agentv1.HealthCheck{
		Path:           p.HealthPath,
		Port:           int32(p.HealthPort),
		TimeoutSeconds: int32(p.HealthTimeoutSeconds),
	}
}

// DeployRunFinished implements agentgw.DeployFinishedHook. It runs for every deploy
// run that finishes, whatever the outcome, and does two things: record where the
// project now stands, and start an automatic rollback when one is warranted.
//
// A rollback is warranted only when all of these hold: the run failed, the project
// opted into AutoRollback, there is an earlier successful run with a known commit,
// that commit differs from the one that just failed, and the failed run was not
// itself a rollback. That last condition is what keeps a broken project from
// rolling back in a loop.
func (h *handler) DeployRunFinished(serverID, runID, status string, exitCode int32, errMsg string) {
	ctx := context.Background()

	run, err := h.store.GetRun(ctx, runID)
	if err != nil {
		h.log.Warn("deploy: rollback check failed to load run", "run_id", runID, "err", err)
		return
	}
	h.recordHealthState(ctx, run, status)

	if status == "succeeded" || run.Trigger == triggerRollback {
		return
	}
	p, err := h.store.Get(ctx, run.ProjectID)
	if err != nil || !p.AutoRollback {
		return
	}
	good, err := h.store.LastSucceededRun(ctx, p.ID)
	if err != nil {
		h.log.Info("deploy: no successful run to roll back to", "project_id", p.ID)
		return
	}
	if good.CommitSha == "" || good.CommitSha == run.CommitSha {
		return // nothing to roll back to, or the failure is on the same commit
	}

	h.log.Warn("deploy: auto-rolling back after failed run",
		"project_id", p.ID, "failed_run_id", runID, "rollback_to_commit", good.CommitSha)

	rollback, err := h.startRun(ctx, p, triggerRollback, good.Branch, good.CommitSha)
	if err != nil {
		h.log.Warn("deploy: auto-rollback failed to start", "project_id", p.ID, "err", err)
		return
	}
	// Audited like any other change to what is running on a server. The actor is the
	// system rather than a user, because nobody asked for this run directly.
	h.audit.Write(ctx, "", "deploy.rollback", "deploy", p.ID, map[string]any{
		"failed_run_id":   runID,
		"rollback_run_id": rollback.ID,
		"commit":          good.CommitSha,
		"branch":          good.Branch,
		"reason":          errMsg,
	})
}

// recordHealthState translates one finished run into the project's standing. It is
// advisory, so a write failure is logged and otherwise ignored.
func (h *handler) recordHealthState(ctx context.Context, run Run, status string) {
	state := HealthDegraded
	switch {
	case status == "succeeded" && run.Trigger == triggerRollback:
		// The app is up, but on the previous commit rather than the newest one.
		state = HealthRolledBack
	case status == "succeeded":
		state = HealthHealthy
	}
	if err := h.store.SetHealthState(ctx, run.ProjectID, state); err != nil {
		h.log.Warn("deploy: health state write failed", "project_id", run.ProjectID, "err", err)
	}
}
