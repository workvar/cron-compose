package runtime

import (
	"context"

	"github.com/croncompose/croncompose/agent/internal/connectors"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// handleConnectorCommand runs one command from the control plane and sends the result
// back correlated by request id.
//
// The result goes out on the DIRECT path, not the durable outbox: a caller is blocked
// on an HTTP request waiting for it, and replaying a stale result after a reconnect
// would resolve nothing while wasting a slot. If the stream is down when we finish,
// dropping the reply is correct, the control plane will have timed out already.
//
// Called with `go` from handleServerMessage, because an apply can take tens of
// seconds and this must not block the agent's receive loop.
func (r *Runtime) handleConnectorCommand(ctx context.Context, cmd *agentv1.ConnectorCommand) {
	if cmd == nil {
		return
	}
	res := r.exec.Execute(ctx, connectors.Command{
		Op:           cmd.GetOp(),
		Kind:         cmd.GetConnectorKind(),
		Instance:     cmd.GetConnectorId(),
		Ref:          cmd.GetRef(),
		Action:       cmd.GetAction(),
		Content:      cmd.GetContent(),
		BaseChecksum: cmd.GetBaseChecksum(),
		DryRun:       cmd.GetDryRun(),
	})

	r.log.Info("connector command",
		"op", cmd.GetOp(), "kind", cmd.GetConnectorKind(),
		"ref", cmd.GetRef(), "action", cmd.GetAction(), "status", res.Status)

	r.sendDirect(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_ConnectorResult{
			ConnectorResult: resultToProto(cmd.GetRequestId(), res),
		},
	})

	// Anything that changed the box invalidates the control plane's discovery cache.
	// Push a fresh sweep so the UI reflects the new state without waiting out the
	// five-minute tick. Read-only ops skip this.
	if mutatingOp(cmd.GetOp()) && !cmd.GetDryRun() {
		go r.discoverConnectors(ctx)
	}
}

func mutatingOp(op string) bool {
	return op == "lifecycle" || op == "apply" || op == "rollback"
}

func resultToProto(requestID string, res connectors.Result) *agentv1.ConnectorResult {
	steps := make([]*agentv1.ConnectorStep, 0, len(res.Steps))
	for _, s := range res.Steps {
		steps = append(steps, &agentv1.ConnectorStep{
			Name:     s.Name,
			Ok:       s.OK,
			Output:   s.Output,
			ExitCode: s.ExitCode,
		})
	}
	return &agentv1.ConnectorResult{
		RequestId:   requestID,
		Status:      res.Status,
		Message:     res.Message,
		Content:     res.Content,
		Checksum:    res.Checksum,
		Steps:       steps,
		PayloadJson: res.Payload,
	}
}
