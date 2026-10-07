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

// SendHostToolsRequest asks an agent to detect or install tools for a chosen OS
// account and waits for HostToolsResult.
func (g *Gateway) SendHostToolsRequest(ctx context.Context, serverID string, op, runAs, tool string) (*agentv1.HostToolsResult, error) {
	requestID := ids.New()
	sub := g.tools.Open(requestID)
	defer g.tools.Close(requestID)

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
		return nil, err
	}

	timeout := toolsDetectTimeout
	if op == "install" {
		timeout = toolsInstallTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case res := <-sub.Result():
		return res, nil
	case <-timer.C:
		return nil, ErrCommandTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
