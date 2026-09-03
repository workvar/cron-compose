package runtime

import (
	"context"
	"os"
	"sync"

	"github.com/croncompose/croncompose/agent/internal/selfupdate"
	"github.com/croncompose/croncompose/agent/internal/sourceupdate"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// updateOnce guards against a second update starting while the first is in flight.
var updateOnce sync.Mutex

// handleUpdate applies an agent self-update, if it is allowed and wanted.
//
// Source updates (a GitHub repo URL, no checksum) clone/build on this machine.
// Binary updates download a pinned artifact. After a binary or standalone-agent
// source swap we exit 0 so the supervisor restarts us. A stack update starts
// update.sh and leaves this process running until that script restarts the stack.
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

	if sourceupdate.IsSource(req.DownloadURL, req.SHA256) {
		r.log.Info("source update starting",
			"from", r.cfg.AgentVersion, "to", req.TargetVersion, "url", req.DownloadURL)
		res, err := sourceupdate.Apply(ctx, r.log, req.DownloadURL, req.TargetVersion, r.cfg.AgentVersion)
		if err != nil {
			r.log.Error("source update failed", "err", err)
			return
		}
		if !res.RestartNow || !u.GetRestart() {
			r.log.Info("source update running", "path", res.Path)
			return
		}
		r.log.Info("exiting so the supervisor restarts the new binary")
		os.Exit(0)
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
	os.Exit(0)
}
