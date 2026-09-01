package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// PendingRequests correlates a ConnectorCommand sent to an agent with the
// ConnectorResult that comes back on the same stream. One waiter per request_id.
//
// It follows the same three rules as TerminalBus, for the same reasons:
//   - RWMutex around a map, RLock on the delivery path;
//   - the value channel is NEVER closed (a late result would panic on a closed
//     channel); waiters select on done instead;
//   - Resolve never blocks, because it runs on the agent's stream receive loop and
//     a stalled deliver would stall every message from that agent.
type PendingRequests struct {
	mu   sync.RWMutex
	subs map[string]*PendingSub
}

// PendingSub is one in-flight request's mailbox.
type PendingSub struct {
	ch   chan *agentv1.ConnectorResult
	done chan struct{}
}

// Result is the channel the caller waits on. At most one value is ever sent.
func (s *PendingSub) Result() <-chan *agentv1.ConnectorResult { return s.ch }

// Done closes when the request is abandoned (caller timed out, gave up, or closed).
func (s *PendingSub) Done() <-chan struct{} { return s.done }

// NewPendingRequests builds an empty registry.
func NewPendingRequests() *PendingRequests {
	return &PendingRequests{subs: map[string]*PendingSub{}}
}

// Open registers a request id and returns its mailbox. The caller must Close it,
// normally with defer, or the entry leaks.
func (p *PendingRequests) Open(requestID string) *PendingSub {
	sub := &PendingSub{
		ch:   make(chan *agentv1.ConnectorResult, 1),
		done: make(chan struct{}),
	}
	p.mu.Lock()
	p.subs[requestID] = sub
	p.mu.Unlock()
	return sub
}

// Close removes the request and signals done. Idempotent.
func (p *PendingRequests) Close(requestID string) {
	p.mu.Lock()
	sub, ok := p.subs[requestID]
	if ok {
		delete(p.subs, requestID)
	}
	p.mu.Unlock()
	if ok {
		close(sub.done)
	}
}

// Resolve hands a result to whoever is waiting on it. A result for an unknown or
// already-abandoned request is dropped, which is the correct behaviour: the HTTP
// request that asked for it is long gone.
func (p *PendingRequests) Resolve(res *agentv1.ConnectorResult) {
	if res == nil {
		return
	}
	p.mu.RLock()
	sub := p.subs[res.GetRequestId()]
	p.mu.RUnlock()
	if sub == nil {
		return
	}
	select {
	case sub.ch <- res:
	case <-sub.done:
	default:
	}
}

// Len reports how many requests are in flight. Test and diagnostics only.
func (p *PendingRequests) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subs)
}
