package runtime

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/croncompose/croncompose/agent/internal/scheduler"
	"github.com/croncompose/croncompose/agent/internal/store"
)

// maxCatchupRuns bounds the "all" policy. An agent that was off for a month with a
// per-minute job would otherwise wake up and try to run forty thousand times, which
// helps nobody and would take the box down more thoroughly than the outage did.
const maxCatchupRuns = 20

// runCatchup fires the windows a job missed while this agent was not running.
//
// Policies:
//
//	skip  do nothing, the missed windows are gone
//	once  run exactly one catch-up, however many windows were missed (the default)
//	all   run every missed window, up to maxCatchupRuns
//
// This is called once at startup, after the cached jobs are loaded and before the
// scheduler starts. It deliberately does not run on every sync: a job definition
// changing is not a missed window.
func (r *Runtime) runCatchup(ctx context.Context, jobs []store.JobDef) {
	state := r.loadState()
	now := time.Now().UTC()

	for _, def := range jobs {
		if !def.Enabled || def.CatchupPolicy == "skip" || def.CatchupPolicy == "" {
			continue
		}
		last := state[def.ID].LastFiredAt
		if last.IsZero() {
			// Never fired: there is no window to have missed. Treat now as the
			// baseline so the next restart can reason about it.
			r.recordFire(def.ID, now)
			continue
		}

		missed := missedWindows(def, last, now)
		if len(missed) == 0 {
			continue
		}

		switch def.CatchupPolicy {
		case "once":
			r.log.Info("catching up one missed window",
				"job_id", def.ID, "missed", len(missed), "since", last)
			r.executeRun(ctx, def.ID, newULID(), "catchup")
		case "all":
			r.log.Info("catching up missed windows",
				"job_id", def.ID, "missed", len(missed), "since", last)
			for range missed {
				r.executeRun(ctx, def.ID, newULID(), "catchup")
			}
		}
	}
}

// missedWindows lists the schedule times between last and now, capped. The cap is
// applied by stopping early rather than by truncating, so a wildly stale timestamp
// cannot make this loop for a long time before returning.
func missedWindows(def store.JobDef, last, now time.Time) []time.Time {
	sched, err := cron.ParseStandard(def.ScheduleCron)
	if err != nil {
		return nil
	}
	loc := scheduler.LoadLocation(def.Timezone)

	var out []time.Time
	cursor := last.In(loc)
	for len(out) < maxCatchupRuns {
		next := sched.Next(cursor)
		if !next.Before(now.In(loc)) {
			break
		}
		out = append(out, next)
		cursor = next
	}
	return out
}

// loadState reads the persisted per-job history, keeping an in-memory copy so the
// hot path (recording a fire) does not read the file every time.
func (r *Runtime) loadState() map[string]store.JobState {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.state == nil {
		r.state = r.store.LoadState()
	}
	out := make(map[string]store.JobState, len(r.state))
	for k, v := range r.state {
		out[k] = v
	}
	return out
}

// recordFire stamps a job as having run now and persists it. Persisting on every fire
// is the point: a crash between a run and the next checkpoint would otherwise make the
// agent think the window was missed and run it again on restart.
func (r *Runtime) recordFire(jobID string, at time.Time) {
	r.stateMu.Lock()
	if r.state == nil {
		r.state = r.store.LoadState()
	}
	r.state[jobID] = store.JobState{LastFiredAt: at.UTC()}
	snapshot := make(map[string]store.JobState, len(r.state))
	for k, v := range r.state {
		snapshot[k] = v
	}
	r.stateMu.Unlock()

	if err := r.store.SaveState(snapshot); err != nil {
		r.log.Warn("could not persist job state", "job_id", jobID, "err", err)
	}
}
