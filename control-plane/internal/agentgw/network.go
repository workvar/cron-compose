package agentgw

import (
	"context"
	"encoding/json"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

const (
	networkDefaultTimeout = 60 * time.Second
	networkPairTimeout    = 3 * time.Minute
	networkScanTimeout    = 90 * time.Second
)

// BeginNetworkRequest opens a pending mailbox and sends the agent request.
func (g *Gateway) BeginNetworkRequest(serverID, op, argsJSON string) (requestID string, sub *PendingNetworkSub, err error) {
	requestID = ids.New()
	sub = g.network.Open(requestID)
	if err := g.registry.Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_NetworkRequest{
			NetworkRequest: &agentv1.NetworkRequest{
				RequestId: requestID,
				Op:        op,
				ArgsJson:  argsJSON,
			},
		},
	}); err != nil {
		g.network.Close(requestID)
		return "", nil, err
	}
	return requestID, sub, nil
}

// CloseNetworkRequest drops a pending network subscription.
func (g *Gateway) CloseNetworkRequest(requestID string) {
	g.network.Close(requestID)
}

// SendNetworkRequest asks an agent to run a network op and waits for NetworkResult.
// Streaming events are discarded; prefer BeginNetworkRequest for SSE/PIN flows.
func (g *Gateway) SendNetworkRequest(ctx context.Context, serverID, op, argsJSON string) (*agentv1.NetworkResult, error) {
	requestID, sub, err := g.BeginNetworkRequest(serverID, op, argsJSON)
	if err != nil {
		return nil, err
	}
	defer g.CloseNetworkRequest(requestID)

	timeout := networkDefaultTimeout
	switch op {
	case "bt_pair":
		timeout = networkPairTimeout
	case "wifi_scan", "bt_scan":
		timeout = networkScanTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case res := <-sub.Result():
			return res, nil
		case <-sub.Events():
			// drain
		case <-timer.C:
			return nil, ErrCommandTimeout
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// SendNetworkPinReply delivers a PIN to an in-flight bt_pair. A separate
// request_id is used so the reply's NetworkResult does not close the pair waiter.
func (g *Gateway) SendNetworkPinReply(ctx context.Context, serverID, forRequestID, pin string) (*agentv1.NetworkResult, error) {
	b, err := json.Marshal(map[string]string{
		"pin":            pin,
		"for_request_id": forRequestID,
	})
	if err != nil {
		return nil, err
	}
	return g.SendNetworkRequest(ctx, serverID, "bt_pin_reply", string(b))
}
