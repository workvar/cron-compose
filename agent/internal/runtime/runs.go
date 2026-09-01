package runtime

import (
	"context"
	"crypto/rand"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/croncompose/croncompose/agent/internal/executor"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// retryBackoff is the pause between attempts. Fixed rather than exponential: these are
// scheduled jobs with a next window coming, so the useful case is "the network blipped,
// try again in a moment", not "back off for an hour".
const retryBackoff = 15 * time.Second

// seqBlock is how much sequence space each attempt reserves. Gaps in seq are harmless;
// collisions are not, because (run_id, stream, seq) is the control plane's key.
const seqBlock = 1_000_000

// onSchedulerFire is the callback the scheduler invokes when a job's cron triggers.
func (r *Runtime) onSchedulerFire(jobID string) {
	r.executeRun(context.Background(), jobID, newULID(), "schedule")
}

// executeRun runs one job execution end to end: concurrency policy, retries,
// RunStarted, streamed LogChunks, RunFinished. Safe to call concurrently for different
// jobs.
//
// Retries reuse the same run id. A retried run is one run that took several attempts,
// not several runs; splitting it would make "this job failed once last night" read as
// "this job failed four times".
func (r *Runtime) executeRun(ctx context.Context, jobID, runID, trigger string) {
	r.jobsMu.RLock()
	def, ok := r.jobs[jobID]
	r.jobsMu.RUnlock()
	if !ok {
		r.log.Warn("fire for unknown job", "job_id", jobID)
		return
	}

	release, admitted := r.admit(ctx, jobID, def.ConcurrencyPolicy)
	if !admitted {
		r.log.Info("skipping run, already in progress",
			"job_id", jobID, "policy", def.ConcurrencyPolicy)
		return
	}
	defer release()

	runCtx, cancel := context.WithCancel(ctx)
	r.trackRun(runID, jobID, cancel)
	defer r.releaseRun(runID, jobID)

	startedAt := time.Now().UTC()
	r.recordFire(jobID, startedAt)
	r.queue(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_RunStarted{RunStarted: &agentv1.RunStarted{
			RunId:        runID,
			JobId:        jobID,
			JobVersionId: def.VersionID,
			Trigger:      trigger,
			StartedAt:    timestamppb.New(startedAt),
		}},
	})

	job := executor.Job{
		Interpreter:     def.Interpreter,
		ScriptBody:      def.ScriptBody,
		Env:             mergeEnv(def.Env, def.Secrets),
		WorkingDir:      def.WorkingDir,
		RunAsUser:       def.RunAsUser,
		TimeoutSeconds:  def.TimeoutSeconds,
		CPUQuotaPercent: def.CPUQuotaPercent,
		MemoryMaxMB:     def.MemoryMaxMB,
		TasksMax:        def.TasksMax,
		IOWeight:        def.IOWeight,
	}

	result := r.runWithRetries(runCtx, runID, job, def.MaxRetries)

	r.queue(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_RunFinished{RunFinished: &agentv1.RunFinished{
			RunId:      runID,
			Status:     result.Status,
			ExitCode:   int32(result.ExitCode),
			FinishedAt: timestamppb.Now(),
			DurationMs: int32(result.DurationMs),
			Error:      result.Err,
		}},
	})
}

// runWithRetries executes the job, retrying a plain failure up to maxRetries times.
//
// A timeout or a cancellation is never retried: a job that ran out of its own time
// budget will do so again, and a cancelled run was cancelled on purpose.
func (r *Runtime) runWithRetries(ctx context.Context, runID string, job executor.Job, maxRetries int) executor.Result {
	attempt := 0
	offset := 0

	for {
		base := offset
		sink := func(stream string, seq int, data []byte) {
			r.queue(&agentv1.AgentMessage{
				Body: &agentv1.AgentMessage_LogChunk{LogChunk: &agentv1.LogChunk{
					RunId:  runID,
					Stream: stream,
					Seq:    int32(base + seq),
					Data:   append([]byte(nil), data...),
				}},
			})
		}

		if attempt > 0 {
			sink("stderr", 0, []byte("croncompose: retry "+strconv.Itoa(attempt)+" of "+strconv.Itoa(maxRetries)))
		}

		result := executor.Run(ctx, job, sink)
		offset += seqBlock

		switch {
		case result.Status == "succeeded",
			result.Status == "timed_out",
			result.Status == "canceled",
			attempt >= maxRetries:
			return result
		}

		attempt++
		select {
		case <-time.After(retryBackoff):
		case <-ctx.Done():
			return result
		}
	}
}

// admit applies the concurrency policy and returns a release function.
//
//	allow  start immediately, however many are already running
//	skip   refuse if anything is running (the default)
//	queue  wait for the running one to finish, then go
//
// The queue path holds a real per-job slot rather than pretending: that is the
// difference between "these must not overlap" and "drop it if it would".
func (r *Runtime) admit(ctx context.Context, jobID, policy string) (func(), bool) {
	if policy == "allow" {
		return func() {}, true
	}

	gate := r.jobGate(jobID)
	if policy == "queue" {
		select {
		case gate <- struct{}{}:
			return func() { <-gate }, true
		case <-ctx.Done():
			return func() {}, false
		}
	}

	// "skip" and anything unrecognised: take the slot or give up immediately.
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, true
	default:
		return func() {}, false
	}
}

// jobGate returns the one-slot channel that serialises a job's runs, creating it on
// first use. One channel per job, kept for the process lifetime: they cost two words
// each, and dropping one mid-flight would let a queued run start early.
func (r *Runtime) jobGate(jobID string) chan struct{} {
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	gate, ok := r.gates[jobID]
	if !ok {
		gate = make(chan struct{}, 1)
		r.gates[jobID] = gate
	}
	return gate
}

func (r *Runtime) trackRun(runID, jobID string, cancel context.CancelFunc) {
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	r.runIndex[runID] = activeRun{jobID: jobID, cancel: cancel}
	r.jobBusy[jobID]++
}

func (r *Runtime) releaseRun(runID, jobID string) {
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	delete(r.runIndex, runID)
	if r.jobBusy[jobID] > 0 {
		r.jobBusy[jobID]--
	}
}

// cancelRun cancels the single in-progress run matching runID. The executor marks the
// run as canceled when its process exits.
func (r *Runtime) cancelRun(runID string) {
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	if a, ok := r.runIndex[runID]; ok {
		a.cancel()
	}
}

func mergeEnv(base, secrets map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(secrets))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range secrets {
		out[k] = v
	}
	return out
}

func newULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}
