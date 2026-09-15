package agentgw

import (
	"context"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// userListTimeout bounds a ListUsers round trip. Reading /etc/passwd (or shelling
// out to dscl on macOS) is fast; this only needs to be generous enough to survive a
// loaded agent, not slow like a config apply.
const userListTimeout = 10 * time.Second

// SendListUsersRequest asks a server's agent for the OS accounts a web terminal
// session could be switched to, and waits for the matching result.
//
// Errors: ErrAgentOffline if no agent stream is registered for the server,
// ErrCommandTimeout if the deadline passes, or ctx.Err() if the caller went away.
func (g *Gateway) SendListUsersRequest(ctx context.Context, serverID string) (*agentv1.ListUsersResult, error) {
	requestID := ids.New()

	sub := g.users.Open(requestID)
	defer g.users.Close(requestID)

	if err := g.registry.Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_ListUsersRequest{
			ListUsersRequest: &agentv1.ListUsersRequest{RequestId: requestID},
		},
	}); err != nil {
		return nil, err
	}

	timer := time.NewTimer(userListTimeout)
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
