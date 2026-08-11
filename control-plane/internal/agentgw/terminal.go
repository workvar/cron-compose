package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// TerminalSub is one browser's subscription to a single terminal session's output.
type TerminalSub struct {
	ch   chan *agentv1.TerminalOutput
	done chan struct{}
}

// Out is the receive side the WebSocket handler reads from.
func (s *TerminalSub) Out() <-chan *agentv1.TerminalOutput { return s.ch }

// Done is closed when the session is torn down.
func (s *TerminalSub) Done() <-chan struct{} { return s.done }

// TerminalBus routes TerminalOutput messages coming off the agent stream back to the
// WebSocket handler that owns each session_id. There is exactly one subscriber per
// session (the browser's socket). It is the terminal analogue of LogBroker, but point to
// point rather than fan-out.
type TerminalBus struct {
	mu   sync.RWMutex
	subs map[string]*TerminalSub
}

// NewTerminalBus returns an empty bus.
func NewTerminalBus() *TerminalBus {
	return &TerminalBus{subs: map[string]*TerminalSub{}}
}

// Open registers a session and returns its subscription. The caller must Close it.
func (b *TerminalBus) Open(sessionID string) *TerminalSub {
	sub := &TerminalSub{
		ch:   make(chan *agentv1.TerminalOutput, 512),
		done: make(chan struct{}),
	}
	b.mu.Lock()
	b.subs[sessionID] = sub
	b.mu.Unlock()
	return sub
}

// Close removes the session and signals done. Idempotent. The channel is intentionally
// never closed, so a late Deliver can never panic on a closed channel; readers select on
// Done instead.
func (b *TerminalBus) Close(sessionID string) {
	b.mu.Lock()
	sub, ok := b.subs[sessionID]
	if ok {
		delete(b.subs, sessionID)
	}
	b.mu.Unlock()
	if ok {
		close(sub.done)
	}
}

// Deliver routes one output message to its session. Non-blocking: a slow browser must
// never stall handleAgentMessage, which runs on the agent's stream receive loop. On
// buffer overflow the message is dropped.
func (b *TerminalBus) Deliver(out *agentv1.TerminalOutput) {
	b.mu.RLock()
	sub := b.subs[out.GetSessionId()]
	b.mu.RUnlock()
	if sub == nil {
		return
	}
	select {
	case sub.ch <- out:
	case <-sub.done:
	default:
	}
}
