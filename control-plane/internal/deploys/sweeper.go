package deploys

import (
	"context"
	"time"
)

// The agent enforces its own run timeout, but an agent that is killed, loses its
// host, or never receives the command reports nothing at all, and the run would sit
// "running" forever: blocking the project's next deploy (see duplicateTrigger) and
// never reaching the code that would roll it back. The sweeper is the backstop.
const (
	sweepEvery = 60 * time.Second
	// Grace on top of the project's own budget, so this never races the agent's
	// timeout and steals a run the agent is about to fail properly itself.
	sweepGrace = 2 * time.Minute
)

// StartSweeper runs the stuck-run sweep until ctx is cancelled. It is safe to run on
// every control-plane instance: marking a run failed is idempotent, and the rollback
// hook it feeds is keyed on the run id.
func (h *handler) StartSweeper(ctx context.Context) {
	ticker := time.NewTicker(sweepEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sweepStuckRuns(ctx)
		}
	}
}

func (h *handler) sweepStuckRuns(ctx context.Context) {
	runs, err := h.store.StuckRuns(ctx, sweepGrace)
	if err != nil {
		h.log.Warn("deploy: stuck run sweep failed", "err", err)
		return
	}
	for _, r := range runs {
		const msg = "no result from the agent before the deploy budget expired"
		if err := h.store.MarkRun(ctx, r.ID, "failed", 1, msg); err != nil {
			h.log.Warn("deploy: could not close out stuck run", "run_id", r.ID, "err", err)
			continue
		}
		h.log.Warn("deploy: closed out stuck run", "run_id", r.ID, "project_id", r.ProjectID)
		// Same path a normal failure takes, so a stuck run still updates the project's
		// health state and still triggers auto-rollback.
		h.DeployRunFinished(r.ServerID, r.ID, "failed", 1, msg)
	}
}
