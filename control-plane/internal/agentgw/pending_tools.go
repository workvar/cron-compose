package agentgw

import (
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// PendingToolRequests correlates HostToolsRequest with streaming events and the
// final HostToolsResult.
type PendingToolRequests struct {
	mu   sync.RWMutex
	subs map[string]*PendingToolsSub
}

// PendingToolsSub is one in-flight tools request's mailbox.
type PendingToolsSub struct {
	ch   chan *agentv1.HostToolsResult
	logs chan *agentv1.HostToolsEvent
	done chan struct{}
}

func (s *PendingToolsSub) Result() <-chan *agentv1.HostToolsResult { return s.ch }
func (s *PendingToolsSub) Logs() <-chan *agentv1.HostToolsEvent    { return s.logs }
func (s *PendingToolsSub) Done() <-chan struct{}                   { return s.done }

func NewPendingToolRequests() *PendingToolRequests {
	return &PendingToolRequests{subs: map[string]*PendingToolsSub{}}
}

func (p *PendingToolRequests) Open(requestID string) *PendingToolsSub {
	sub := &PendingToolsSub{
		ch:   make(chan *agentv1.HostToolsResult, 1),
		logs: make(chan *agentv1.HostToolsEvent, 64),
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

// PushLog forwards a live install/uninstall chunk to any waiter. Drops the
// oldest buffered event if the consumer is slow so the agent stream never
// blocks on a full channel.
func (p *PendingToolRequests) PushLog(ev *agentv1.HostToolsEvent) {
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
	case sub.logs <- ev:
	case <-sub.done:
	default:
		// Drop one stale event, then try once more.
		select {
		case <-sub.logs:
		default:
		}
		select {
		case sub.logs <- ev:
		default:
		}
	}
}
