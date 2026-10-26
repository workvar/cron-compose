package runtime

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/croncompose/croncompose/agent/internal/autoupdate"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// autoUpdateFirstCheck delays the first check so a freshly restarted agent (often
// one that just updated) reconnects and reports in before looking again.
const autoUpdateFirstCheck = 3 * time.Minute

// autoUpdateLoop polls GitHub for a newer release and, when one exists, installs it
// exactly as if the control plane had offered it. Jitter keeps a fleet from hitting
// GitHub (and restarting) in lockstep.
func (r *Runtime) autoUpdateLoop(ctx context.Context) {
	if !r.cfg.AutoUpdate || !r.cfg.SelfUpdate || r.cfg.UpdateRepo == "" {
		return
	}
	interval := r.cfg.AutoUpdateInterval
	if interval < time.Hour {
		interval = time.Hour
	}
	r.log.Info("auto-update enabled", "repo", r.cfg.UpdateRepo, "every", interval.String())
	wait := autoUpdateFirstCheck + jitter(time.Minute*5)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		r.checkForUpdate(ctx)
		wait = interval + jitter(interval/10)
	}
}

func (r *Runtime) checkForUpdate(ctx context.Context) {
	tag, err := autoupdate.Latest(ctx, r.cfg.UpdateRepo)
	if err != nil {
		r.log.Debug("auto-update check failed", "err", err)
		return
	}
	if !autoupdate.Newer(r.cfg.AgentVersion, tag) {
		return
	}
	r.log.Info("auto-update found a newer release", "current", r.cfg.AgentVersion, "latest", tag)
	r.handleUpdate(ctx, &agentv1.UpdateAgent{
		TargetVersion: tag,
		DownloadUrl:   "https://github.com/" + r.cfg.UpdateRepo,
		Restart:       true,
	})
}

func jitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(max)))
}
