package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// PendingUserRequests correlates a ListUsersRequest sent to an agent with the
// ListUsersResult that comes back on the same stream.
//
// This is PendingRequests' twin, kept as a separate type rather than a generic one:
// the two result types (ConnectorResult, ListUsersResult) don't share a wire shape,
// and the request/reply traffic here is low-volume (one round trip per terminal
// "who can I run as" lookup), so the duplication buys clarity over a shared type
// parameter that would otherwise be threaded through both call sites. Same three
// rules as PendingRequests, for the same reasons: RWMutex with RLock on delivery,
// the value channel is never closed, Resolve never blocks.
type PendingUserRequests struct {
	mu   sync.RWMutex
	subs map[string]*PendingUsersSub
}

// PendingUsersSub is one in-flight request's mailbox.
type PendingUsersSub struct {
	ch   chan *agentv1.ListUsersResult
	done chan struct{}
}

// Result is the channel the caller waits on. At most one value is ever sent.
func (s *PendingUsersSub) Result() <-chan *agentv1.ListUsersResult { return s.ch }

// Done closes when the request is abandoned (caller timed out, gave up, or closed).
func (s *PendingUsersSub) Done() <-chan struct{} { return s.done }

// NewPendingUserRequests builds an empty registry.
func NewPendingUserRequests() *PendingUserRequests {
	return &PendingUserRequests{subs: map[string]*PendingUsersSub{}}
}

// Open registers a request id and returns its mailbox. The caller must Close it,
// normally with defer, or the entry leaks.
func (p *PendingUserRequests) Open(requestID string) *PendingUsersSub {
	sub := &PendingUsersSub{
		ch:   make(chan *agentv1.ListUsersResult, 1),
		done: make(chan struct{}),
	}
	p.mu.Lock()
	p.subs[requestID] = sub
	p.mu.Unlock()
	return sub
}

// Close removes the request and signals done. Idempotent.
func (p *PendingUserRequests) Close(requestID string) {
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
// already-abandoned request is dropped, which is correct: the HTTP request that
// asked for it is long gone.
func (p *PendingUserRequests) Resolve(res *agentv1.ListUsersResult) {
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
