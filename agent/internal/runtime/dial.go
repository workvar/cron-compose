package runtime

import (
	"context"

	"github.com/croncompose/croncompose/agent/internal/transport"
)

// connMode is how this agent authenticates: AGENT_GRPC_MODE wins over the mode the
// control plane saved at enrollment.
func (r *Runtime) connMode() string {
	return transport.ResolveMode(r.cfg.GRPCMode, r.ident.GRPCMode)
}

// dial opens the connection in the agent's mode. Edge mode sends the per-server secret
// over ordinary TLS instead of presenting a client certificate.
func (r *Runtime) dial(ctx context.Context, addr string) (*transport.Client, error) {
	if r.connMode() == transport.ModeEdge {
		return transport.DialEdge(ctx, addr, transport.EdgeOptions{
			ServerID: r.ident.ServerID,
			Secret:   r.ident.AgentSecret,
		})
	}
	return transport.Dial(ctx, addr, r.tlsCfg)
}
