package runtime

import (
	"context"
	"os"
	"sync"

	"github.com/croncompose/croncompose/agent/internal/selfupdate"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// updateOnce guards against a second update starting while the first is downloading.
// The control plane sends UpdateAgent on every connect until the version matches, and
// a flapping connection would otherwise start several downloads at once.
var updateOnce sync.Mutex

// handleUpdate applies an agent self-update, if it is allowed and wanted.
//
// The exit at the end is the update: the supervisor (pm2, systemd, docker) restarts
// the process and it comes back on the new binary. Exiting 0 matters, because a
// non-zero exit reads as a crash and some supervisors back off after enough of them.
//
// If the agent is not supervised, AGENT_SELF_UPDATE=0 turns this off entirely and the
// operator updates the binary themselves, which is the right call for a hand-managed
// box.
func (r *Runtime) handleUpdate(ctx context.Context, u *agentv1.UpdateAgent) {
	if u == nil {
		return
	}
	if !r.cfg.SelfUpdate {
		r.log.Info("self-update offered but disabled on this agent",
			"target_version", u.GetTargetVersion())
		return
	}

	if !updateOnce.TryLock() {
		r.log.Debug("self-update already in progress")
		return
	}
	defer updateOnce.Unlock()

	req := selfupdate.Request{
		TargetVersion: u.GetTargetVersion(),
		DownloadURL:   u.GetDownloadUrl(),
		SHA256:        u.GetSha256(),
	}
	if err := req.Validate(r.cfg.AgentVersion); err != nil {
		r.log.Info("self-update skipped", "reason", err)
		return
	}

	r.log.Info("self-update starting",
		"from", r.cfg.AgentVersion, "to", req.TargetVersion, "url", req.DownloadURL)

	path, err := selfupdate.Apply(ctx, req, r.cfg.AgentVersion)
	if err != nil {
		r.log.Error("self-update failed", "err", err)
		return
	}
	r.log.Info("self-update installed", "path", path, "version", req.TargetVersion)

	if !u.GetRestart() {
		r.log.Info("new binary is in place; it takes effect on the next restart")
		return
	}

	r.log.Info("exiting so the supervisor restarts the new binary")
	// Give the log line a chance to flush through the supervisor's pipe.
	os.Exit(0)
}
