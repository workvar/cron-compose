package agentgw

import (
	"context"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// toolsDetectTimeout bounds a detect round trip.
const toolsDetectTimeout = 45 * time.Second

// toolsInstallTimeout bounds a toolchain install (nvm / go tarball / apt).
const toolsInstallTimeout = 15 * time.Minute

// BeginHostToolsRequest opens a pending mailbox, sends the agent request, and
// returns the subscription. The caller must Close the request id when done.
// Live log chunks arrive on sub.Logs(); the final HostToolsResult on sub.Result().
func (g *Gateway) BeginHostToolsRequest(serverID, op, runAs, tool string) (requestID string, sub *PendingToolsSub, err error) {
	requestID = ids.New()
	sub = g.tools.Open(requestID)
	if err := g.registry.Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_HostToolsRequest{
			HostToolsRequest: &agentv1.HostToolsRequest{
				RequestId: requestID,
				Op:        op,
				RunAs:     runAs,
				Tool:      tool,
			},
		},
	}); err != nil {
		g.tools.Close(requestID)
		return "", nil, err
	}
	return requestID, sub, nil
}

// CloseHostToolsRequest drops a pending tools subscription.
func (g *Gateway) CloseHostToolsRequest(requestID string) {
	g.tools.Close(requestID)
}

// SendHostToolsRequest asks an agent to detect or install tools for a chosen OS
// account and waits for HostToolsResult. Install/uninstall log chunks are
// discarded; prefer BeginHostToolsRequest when streaming to a client.
func (g *Gateway) SendHostToolsRequest(ctx context.Context, serverID string, op, runAs, tool string) (*agentv1.HostToolsResult, error) {
	requestID, sub, err := g.BeginHostToolsRequest(serverID, op, runAs, tool)
	if err != nil {
		return nil, err
	}
	defer g.CloseHostToolsRequest(requestID)

	timeout := toolsDetectTimeout
	if op == "install" || op == "uninstall" {
		timeout = toolsInstallTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case res := <-sub.Result():
			return res, nil
		case <-sub.Logs():
			// drain streaming chunks for non-SSE callers
		case <-timer.C:
			return nil, ErrCommandTimeout
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
