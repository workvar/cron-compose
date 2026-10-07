package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// PendingToolRequests correlates HostToolsRequest with HostToolsResult.
type PendingToolRequests struct {
	mu   sync.RWMutex
	subs map[string]*PendingToolsSub
}

// PendingToolsSub is one in-flight tools request's mailbox.
type PendingToolsSub struct {
	ch   chan *agentv1.HostToolsResult
	done chan struct{}
}

func (s *PendingToolsSub) Result() <-chan *agentv1.HostToolsResult { return s.ch }
func (s *PendingToolsSub) Done() <-chan struct{}                   { return s.done }

func NewPendingToolRequests() *PendingToolRequests {
	return &PendingToolRequests{subs: map[string]*PendingToolsSub{}}
}

func (p *PendingToolRequests) Open(requestID string) *PendingToolsSub {
	sub := &PendingToolsSub{
		ch:   make(chan *agentv1.HostToolsResult, 1),
		done: make(chan struct{}),
	}
	p.mu.Lock()
	p.subs[requestID] = sub
	p.mu.Unlock()
	return sub
}

func (p *PendingToolRequests) Close(requestID string) {
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

func (p *PendingToolRequests) Resolve(res *agentv1.HostToolsResult) {
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
