package runtime

import (
	"context"

	"github.com/croncompose/croncompose/agent/internal/scheduler"
	"github.com/croncompose/croncompose/agent/internal/store"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// handleServerMessage routes one inbound ServerMessage.
func (r *Runtime) handleServerMessage(ctx context.Context, msg *agentv1.ServerMessage) {
	switch body := msg.GetBody().(type) {
	case *agentv1.ServerMessage_SyncJobs:
		r.applySync(body.SyncJobs)
	case *agentv1.ServerMessage_RunNow:
		go r.executeRun(ctx, body.RunNow.GetJobId(), body.RunNow.GetRunId(), "manual")
	case *agentv1.ServerMessage_CancelRun:
		r.cancelRun(body.CancelRun.GetRunId())
	case *agentv1.ServerMessage_TerminalInput:
		r.terminals.Handle(body.TerminalInput)
	}
}

// applySync replaces (full snapshot) or merges (delta) the cached job set, then writes
// the cache to disk and re-applies the scheduler.
func (r *Runtime) applySync(s *agentv1.SyncJobs) {
	r.jobsMu.Lock()
	if s.GetFullSnapshot() {
		r.jobs = map[string]store.JobDef{}
	}
	for _, d := range s.GetUpsert() {
		r.jobs[d.GetId()] = protoToStore(d)
	}
	for _, id := range s.GetDelete() {
		delete(r.jobs, id)
	}
	// Build a snapshot for scheduler/store outside the lock to keep the critical
	// section short.
	all := make([]store.JobDef, 0, len(r.jobs))
	for _, j := range r.jobs {
		all = append(all, j)
	}
	r.jobsMu.Unlock()

	if err := r.store.SaveJobs(all); err != nil {
		r.log.Warn("save jobs failed", "err", err)
	}

	if err := r.sched.Apply(toSchedulerJobs(all)); err != nil {
		r.log.Warn("scheduler apply failed", "err", err)
	}

	r.queue(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_ConfigAck{ConfigAck: &agentv1.ConfigAck{
			AppliedCursor: s.GetCursor(),
		}},
	})
	r.log.Info("synced jobs", "count", len(all), "cursor", s.GetCursor())
}

// applyCachedJobs seeds runtime state from disk before the first SyncJobs arrives.
func (r *Runtime) applyCachedJobs(cached []store.JobDef) {
	r.jobsMu.Lock()
	for _, j := range cached {
		r.jobs[j.ID] = j
	}
	r.jobsMu.Unlock()
	if err := r.sched.Apply(toSchedulerJobs(cached)); err != nil {
		r.log.Warn("scheduler apply (cached) failed", "err", err)
	}
}

func protoToStore(d *agentv1.JobDef) store.JobDef {
	return store.JobDef{
		ID:                d.GetId(),
		VersionID:         d.GetVersionId(),
		Name:              d.GetName(),
		Interpreter:       d.GetInterpreter(),
		ScriptBody:        d.GetScriptBody(),
		ScheduleCron:      d.GetScheduleCron(),
		Timezone:          d.GetTimezone(),
		Enabled:           d.GetEnabled(),
		TimeoutSeconds:    int(d.GetTimeoutSeconds()),
		ConcurrencyPolicy: d.GetConcurrencyPolicy(),
		CatchupPolicy:     d.GetCatchupPolicy(),
		MaxRetries:        int(d.GetMaxRetries()),
		WorkingDir:        d.GetWorkingDir(),
		RunAsUser:         d.GetRunAsUser(),
		Env:               d.GetEnv(),
		Secrets:           d.GetSecrets(),
		CPUQuotaPercent:   int(d.GetCpuQuotaPercent()),
		MemoryMaxMB:       int(d.GetMemoryMaxMb()),
	}
}

func toSchedulerJobs(in []store.JobDef) []scheduler.JobDef {
	out := make([]scheduler.JobDef, 0, len(in))
	for _, j := range in {
		out = append(out, scheduler.JobDef{
			ID:           j.ID,
			ScheduleCron: j.ScheduleCron,
			Timezone:     j.Timezone,
			Enabled:      j.Enabled,
		})
	}
	return out
}
