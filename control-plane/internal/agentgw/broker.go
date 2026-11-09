package agentgw

import (
	"sync"

	"github.com/croncompose/croncompose/control-plane/internal/metrics"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// LogEvent is what subscribers receive. Either a LogChunk or, when the run finishes,
// a Finished marker so the SSE endpoint can close cleanly.
type LogEvent struct {
	Chunk    *agentv1.LogChunk    // non-nil for log chunks
	Finished *agentv1.RunFinished // non-nil for run-finished events
}

// subscriberBuffer is how many events a subscriber may fall behind before chunks
// are dropped. 64 was easy to overrun during a parallel build, which showed up as
// gaps in the live terminal until the next snapshot poll filled them in.
const subscriberBuffer = 1024

// LogBroker fans out incoming log chunks (and the terminal RunFinished) to any number
// of subscribers keyed by run_id. Used by the SSE endpoint in the REST API.
type LogBroker struct {
	mu   sync.RWMutex
	subs map[string]map[chan LogEvent]struct{}
}

// NewLogBroker returns an empty broker.
func NewLogBroker() *LogBroker {
	return &LogBroker{subs: map[string]map[chan LogEvent]struct{}{}}
}

// Subscribe returns a channel of log events for the run. Caller must Unsubscribe.
func (b *LogBroker) Subscribe(runID string) chan LogEvent {
	ch := make(chan LogEvent, subscriberBuffer)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[runID] == nil {
		b.subs[runID] = map[chan LogEvent]struct{}{}
	}
	b.subs[runID][ch] = struct{}{}
	metrics.LogSubscribers.Inc()
	return ch
}

// Unsubscribe drops a subscriber and closes its channel.
func (b *LogBroker) Unsubscribe(runID string, ch chan LogEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if set, ok := b.subs[runID]; ok {
		if _, ok := set[ch]; ok {
			delete(set, ch)
			close(ch)
			metrics.LogSubscribers.Dec()
		}
		if len(set) == 0 {
			delete(b.subs, runID)
		}
	}
}

// Publish sends a chunk to every subscriber of the run. Non-blocking: a slow
// subscriber that fills its buffer drops events.
func (b *LogBroker) Publish(runID string, c *agentv1.LogChunk) {
	b.fanout(runID, LogEvent{Chunk: c})
}

// PublishFinished signals that the run is done so SSE clients can close. It is
// never dropped: when a subscriber's buffer is full, the oldest queued chunk is
// discarded to make room (it is already stored and the page refetches the log).
// Losing this event instead left the run page saying "running" indefinitely.
func (b *LogBroker) PublishFinished(runID string, r *agentv1.RunFinished) {
	ev := LogEvent{Finished: r}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[runID] {
		for {
			select {
			case ch <- ev:
			default:
				select {
				case <-ch:
				default:
				}
				continue
			}
			break
		}
	}
}

func (b *LogBroker) fanout(runID string, ev LogEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[runID] {
		select {
		case ch <- ev:
		default:
		}
	}
}
