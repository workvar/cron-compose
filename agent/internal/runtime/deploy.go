package runtime

import (
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
	})
}
