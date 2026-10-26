package runtime

import (
	"context"
	"sync"

	"github.com/croncompose/croncompose/agent/internal/platform"
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

	report := func(phase, detail string, percent int) {
		r.reportUpdate(req.TargetVersion, phase, detail, percent)
	}

	// Windows installs never compile from source: the same offer is answered with
	// the release's prebuilt, checksum-pinned .exe.
	if platform.IsWindows() && sourceupdate.IsSource(req.DownloadURL, req.SHA256) {
		r.log.Info("release binary update starting",
			"from", r.cfg.AgentVersion, "to", req.TargetVersion)
		report("offered", "Starting the update", 6)
		path, err := sourceupdate.ApplyReleaseBinary(ctx, r.log, req.DownloadURL, req.TargetVersion, r.cfg.AgentVersion, report)
		if err != nil {
			r.log.Error("release binary update failed", "err", err)
			report("failed", err.Error(), 0)
			return
		}
		r.finishBinaryUpdate(u, path, req.TargetVersion, report)
		return
	}

	if sourceupdate.IsSource(req.DownloadURL, req.SHA256) {
		r.log.Info("source update starting",
			"from", r.cfg.AgentVersion, "to", req.TargetVersion, "url", req.DownloadURL)
		report("offered", "Starting the source update", 6)
		res, err := sourceupdate.Apply(ctx, r.log, req.DownloadURL, req.TargetVersion, r.cfg.AgentVersion, report)
		if err != nil {
			r.log.Error("source update failed", "err", err)
			report("failed", err.Error(), 0)
			return
		}
		if !res.RestartNow || !u.GetRestart() {
			r.log.Info("source update running", "path", res.Path)
			return
		}
		r.log.Info("exiting so the supervisor restarts the new binary")
		report("restarting", "Restarting the agent on the new binary", 90)
		selfupdate.ClearPinnedAgentVersion()
		selfupdate.FlushAndExit()
	}

	if err := req.Validate(r.cfg.AgentVersion); err != nil {
		r.log.Info("self-update skipped", "reason", err)
		return
	}

	r.log.Info("self-update starting",
		"from", r.cfg.AgentVersion, "to", req.TargetVersion, "url", req.DownloadURL)
	report("downloading", "Downloading the new agent binary", 20)

	path, err := selfupdate.Apply(ctx, req, r.cfg.AgentVersion)
	if err != nil {
		r.log.Error("self-update failed", "err", err)
		report("failed", err.Error(), 0)
		return
	}
	r.finishBinaryUpdate(u, path, req.TargetVersion, report)
}

// finishBinaryUpdate reports the swap and, when asked, exits so the supervisor
// (systemd, launchd, or the Windows Service Control Manager) restarts the agent on
// the new binary.
func (r *Runtime) finishBinaryUpdate(u *agentv1.UpdateAgent, path, version string, report func(string, string, int)) {
	r.log.Info("self-update installed", "path", path, "version", version)
	report("installing", "Installed the new agent binary", 80)

	if !u.GetRestart() {
		r.log.Info("new binary is in place; it takes effect on the next restart")
		report("done", "New binary is in place; it takes effect on the next restart", 100)
		return
	}

	r.log.Info("exiting so the supervisor restarts the new binary")
	report("restarting", "Restarting the agent on the new binary", 90)
	selfupdate.ClearPinnedAgentVersion()
	selfupdate.FlushAndExit()
}

func (r *Runtime) reportUpdate(target, phase, detail string, percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	r.sendDirect(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_UpdateProgress{UpdateProgress: &agentv1.UpdateProgress{
			TargetVersion: target,
			Phase:         phase,
			Detail:        detail,
			Percent:       int32(percent),
		}},
	})
}
