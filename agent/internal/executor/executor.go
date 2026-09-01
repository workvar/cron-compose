// Package executor runs a job's script on the local machine with timeout, log capture,
// optional resource limits (CPU, memory, tasks, IO weight) via systemd-run, and
// optional user switching.
package executor

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// LogSink receives chunked output as a job runs. seq increments per stream.
type LogSink func(stream string, seq int, data []byte)

// Job is the minimum the executor needs to run something.
type Job struct {
	Interpreter     string // bash | sh | python3 | node
	ScriptBody      string
	Env             map[string]string
	WorkingDir      string
	RunAsUser       string // empty = run as the agent's own user
	TimeoutSeconds  int
	CPUQuotaPercent int // 0 = unlimited; 100 = one full core
	MemoryMaxMB     int // 0 = unlimited
	TasksMax        int // 0 = unlimited; caps forks, which is what stops a fork bomb
	IOWeight        int // 0 = default; 1..10000, relative block IO priority
}

// Result summarizes how a run ended.
type Result struct {
	Status     string // succeeded | failed | timed_out | canceled
	ExitCode   int
	DurationMs int
	Err        string
	// Notes carries anything the operator should know that is not a failure, such as
	// a resource limit that could not be enforced. Emitted into the run log.
	Notes []string
}

// Run executes the job, streams logs through sink, and returns the result.
func Run(ctx context.Context, j Job, sink LogSink) Result {
	start := time.Now()
	timeout := time.Duration(j.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	interp := j.Interpreter
	if interp == "" {
		interp = "bash"
	}

	cred, err := osuser.Resolve(j.RunAsUser)
	if err != nil {
		// Refuse rather than silently running as the wrong user. A job that says it
		// runs as `deploy` must never quietly run as root.
		return Result{
			Status:     "failed",
			DurationMs: int(time.Since(start).Milliseconds()),
			Err:        "run_as_user: " + err.Error(),
		}
	}

	var notes []string
	prog, args := buildCommand(interp, j)
	if j.limited() && prog != "systemd-run" {
		notes = append(notes, "resource limits requested but systemd-run is not available; running unlimited")
	}
	args = withUser(prog, args, cred)

	cmd := exec.CommandContext(runCtx, prog, args...)
	cmd.Dir = j.WorkingDir
	if cmd.Dir == "" && cred != nil {
		cmd.Dir = cred.Home
	}
	cmd.Env = buildEnv(j.Env, cred)
	// systemd-run does its own user switching via --uid; putting a credential on the
	// systemd-run process itself would stop it from being able to talk to systemd.
	if prog != "systemd-run" {
		cmd.SysProcAttr = cred.SysProcAttr()
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	for i, n := range notes {
		sink("stderr", i, []byte("croncompose: "+n))
	}

	if err := cmd.Start(); err != nil {
		return Result{
			Status:     "failed",
			DurationMs: int(time.Since(start).Milliseconds()),
			Err:        startError(err, cred),
			Notes:      notes,
		}
	}

	// Both pumps must finish before Wait returns, or trailing output is lost: Wait
	// closes the pipes, and a scanner mid-read then sees EOF and drops the line.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); pumpLines(stdout, "stdout", sink, 0) }()
	go func() { defer wg.Done(); pumpLines(stderr, "stderr", sink, len(notes)) }()
	wg.Wait()

	err = cmd.Wait()
	dur := int(time.Since(start).Milliseconds())

	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		return Result{Status: "timed_out", DurationMs: dur, Err: "timeout exceeded", Notes: notes}
	case errors.Is(ctx.Err(), context.Canceled):
		return Result{Status: "canceled", DurationMs: dur, Err: "canceled", Notes: notes}
	case err == nil:
		return Result{Status: "succeeded", ExitCode: 0, DurationMs: dur, Notes: notes}
	}

	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, okAssert := ee.Sys().(syscall.WaitStatus); okAssert && ws.Signaled() {
			return Result{Status: "failed", ExitCode: -int(ws.Signal()), DurationMs: dur,
				Err: ws.Signal().String(), Notes: notes}
		}
		return Result{Status: "failed", ExitCode: ee.ExitCode(), DurationMs: dur,
			Err: err.Error(), Notes: notes}
	}
	return Result{Status: "failed", DurationMs: dur, Err: err.Error(), Notes: notes}
}

// pumpLines streams one pipe line by line. seqStart offsets the sequence numbers past
// any note lines already emitted on this stream, so the (stream, seq) pair the control
// plane uses as a primary key stays unique.
func pumpLines(r io.Reader, stream string, sink LogSink, seqStart int) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	seq := seqStart
	for scanner.Scan() {
		sink(stream, seq, append([]byte{}, scanner.Bytes()...))
		seq++
	}
}

// startError adds the missing context to the most confusing failure here: "operation
// not permitted" from a credential the kernel refused.
func startError(err error, cred *osuser.Credential) string {
	if cred != nil && errors.Is(err, syscall.EPERM) {
		return "could not switch to user " + cred.Username + ": " + err.Error() +
			" (the agent must run as root to change users)"
	}
	return err.Error()
}
