package runtime

import (
	"github.com/croncompose/croncompose/agent/internal/deploy"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func (r *Runtime) initDeploys() {
	r.deploys = deploy.NewManager(r.log, func(ev *agentv1.DeployEvent) {
		r.sendDirect(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_DeployEvent{DeployEvent: ev},
		})
	})
}
