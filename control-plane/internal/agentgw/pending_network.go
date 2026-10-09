package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// PendingNetworkRequests correlates NetworkRequest with streaming events and the
// final NetworkResult.
type PendingNetworkRequests struct {
	mu   sync.RWMutex
	subs map[string]*PendingNetworkSub
}

// PendingNetworkSub is one in-flight network request's mailbox.
type PendingNetworkSub struct {
	ch     chan *agentv1.NetworkResult
	events chan *agentv1.NetworkEvent
	done   chan struct{}
}

func (s *PendingNetworkSub) Result() <-chan *agentv1.NetworkResult { return s.ch }
func (s *PendingNetworkSub) Events() <-chan *agentv1.NetworkEvent  { return s.events }
func (s *PendingNetworkSub) Done() <-chan struct{}                 { return s.done }

func NewPendingNetworkRequests() *PendingNetworkRequests {
	return &PendingNetworkRequests{subs: map[string]*PendingNetworkSub{}}
}

func (p *PendingNetworkRequests) Open(requestID string) *PendingNetworkSub {
	sub := &PendingNetworkSub{
		ch:     make(chan *agentv1.NetworkResult, 1),
		events: make(chan *agentv1.NetworkEvent, 64),
		done:   make(chan struct{}),
	}
	p.mu.Lock()
	p.subs[requestID] = sub
	p.mu.Unlock()
	return sub
}

func (p *PendingNetworkRequests) Close(requestID string) {
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

func (p *PendingNetworkRequests) Resolve(res *agentv1.NetworkResult) {
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

func (p *PendingNetworkRequests) PushEvent(ev *agentv1.NetworkEvent) {
	if ev == nil {
		return
	}
	p.mu.RLock()
	sub := p.subs[ev.GetRequestId()]
	p.mu.RUnlock()
	if sub == nil {
		return
	}
	select {
	case sub.events <- ev:
	case <-sub.done:
	default:
		select {
		case <-sub.events:
		default:
		}
		select {
		case sub.events <- ev:
		default:
		}
	}
}
