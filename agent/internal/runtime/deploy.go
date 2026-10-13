package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/deploy"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func (r *Runtime) initDeploys() {
	// Deploy progress goes through the durable outbox (same path as job LogChunks),
	// not sendDirect: live install output must survive a brief stream blip, and the
	// direct buffer silently drops when full — which left the UI stuck on
	// "(no output yet)" while the run was still marked running.
	r.deploys = deploy.NewManager(r.log, func(ev *agentv1.DeployEvent) {
		r.queue(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_DeployEvent{DeployEvent: ev},
		})
		// After a process starts (or the run finishes), refresh pm2/systemd inventory
		// so the Deploy board sees the real ecosystem process name as online.
		kind := ev.GetKind()
		phase := ev.GetPhase()
		data := string(ev.GetData())
		if kind == "finished" || kind == "error" ||
			((phase == "run" || phase == "start") && strings.Contains(data, ": done ")) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				r.discoverConnectors(ctx)
			}()
		}
	})
}
