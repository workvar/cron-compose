package agentgw

import (
	"context"
	"errors"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// ErrCommandTimeout is returned when the agent accepted a command but never sent a
// result. The command may still have taken effect on the box, so callers should
// record the operation as `timeout` rather than `failed`.
var ErrCommandTimeout = errors.New("agent did not answer in time")

// DefaultCommandTimeout bounds a connector round trip. Config validation and service
// restarts are the slow cases; anything past this is treated as a lost reply.
const DefaultCommandTimeout = 30 * time.Second

// SendConnectorCommand pushes one command to a server's agent and waits for the
// matching result.
//
// The request id is minted here so the caller cannot accidentally reuse one. On
// return the pending entry is always released, including on timeout, so a late
// result is dropped instead of resolving somebody else's request.
//
// Errors: ErrAgentOffline if no agent stream is registered for the server,
// ErrCommandTimeout if the deadline passes, or ctx.Err() if the caller went away.
func (g *Gateway) SendConnectorCommand(ctx context.Context, serverID string, cmd *agentv1.ConnectorCommand) (*agentv1.ConnectorResult, error) {
	if cmd.GetRequestId() == "" {
		cmd.RequestId = ids.New()
	}
	requestID := cmd.GetRequestId()

	sub := g.pending.Open(requestID)
	defer g.pending.Close(requestID)

	if err := g.registry.Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_ConnectorCommand{ConnectorCommand: cmd},
	}); err != nil {
		return nil, err
	}

	timer := time.NewTimer(DefaultCommandTimeout)
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

// NewRequestID mints an id callers can log before the command goes out, so an
// operation row exists even if the send fails.
func NewRequestID() string { return ids.New() }
