package runtime

import (
	"github.com/croncompose/croncompose/agent/internal/terminal"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// initTerminals constructs the terminal manager and wires its output to the ephemeral
// direct-send path rather than the durable outbox: terminal bytes are real-time and must
// never be persisted or replayed on reconnect.
func (r *Runtime) initTerminals() {
	r.terminals = terminal.NewManager(r.log, func(out *agentv1.TerminalOutput) {
		r.sendDirect(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_TerminalOutput{TerminalOutput: out},
		})
	})
}

// sendDirect queues an EPHEMERAL AgentMessage for the drain loop to write on the stream,
// bypassing the persistent outbox. Best-effort: if the buffer is full or the stream is
// down, the message is dropped. The drain loop remains the sole sender on the stream.
func (r *Runtime) sendDirect(msg *agentv1.AgentMessage) {
	select {
	case r.direct <- msg:
	default:
		r.log.Warn("terminal direct-send buffer full; dropping output")
	}
}
