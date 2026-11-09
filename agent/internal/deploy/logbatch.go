package deploy

import (
	"sync"
	"time"
)

// Script output arrives in many small reads (often one line each). Sending each
// read as its own DeployEvent costs a bbolt fsync on the agent, a gRPC message,
// a Postgres insert on the control plane, and a React re-render in the browser.
// On a Raspberry Pi SD card that chain was slower than the build itself, so the
// run board fell minutes behind and the installer blocked on a full pipe.
//
// logBatcher coalesces output and flushes it when it has been buffered for
// logFlushDelay or has grown past logFlushBytes, whichever comes first. The
// delay is short enough that the terminal still feels live.
const (
	logFlushDelay = 100 * time.Millisecond
	logFlushBytes = 64 << 10
)

type logBatcher struct {
	flush func([]byte)
	delay time.Duration
	max   int

	// flushMu orders flushes so a timer flush and a size flush cannot reorder
	// output. mu only guards the buffer, so writers rarely wait on the network.
	flushMu sync.Mutex
	mu      sync.Mutex
	buf     []byte
	gen     uint64
	armed   bool
	closed  bool
}

func newLogBatcher(delay time.Duration, max int, flush func([]byte)) *logBatcher {
	if delay <= 0 {
		delay = logFlushDelay
	}
	if max <= 0 {
		max = logFlushBytes
	}
	return &logBatcher{flush: flush, delay: delay, max: max}
}

// Write buffers p. It flushes inline only when the buffer is full.
func (b *logBatcher) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.buf = append(b.buf, p...)
	full := len(b.buf) >= b.max
	if !full && !b.armed {
		b.armed = true
		gen := b.gen
		time.AfterFunc(b.delay, func() { b.flushGen(gen) })
	}
	b.mu.Unlock()
	if full {
		b.Flush()
	}
}

// flushGen is the timer path. A timer armed before an inline flush is stale and
// does nothing, so the next batch still gets its full delay.
func (b *logBatcher) flushGen(gen uint64) {
	b.mu.Lock()
	stale := gen != b.gen
	b.mu.Unlock()
	if !stale {
		b.Flush()
	}
}

// Flush sends whatever is buffered now.
func (b *logBatcher) Flush() {
	b.flushMu.Lock()
	defer b.flushMu.Unlock()
	b.mu.Lock()
	out := b.buf
	b.buf = nil
	b.gen++
	b.armed = false
	b.mu.Unlock()
	if len(out) > 0 {
		b.flush(out)
	}
}

// Close flushes the remainder and drops later writes.
func (b *logBatcher) Close() {
	b.Flush()
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
}
