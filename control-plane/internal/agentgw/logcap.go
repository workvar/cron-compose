package agentgw

import (
	"sync"
)

// DefaultRunLogMaxBytes is how much output one run may store before the rest is
// dropped. Five megabytes is roughly fifty thousand lines: far more than anyone reads,
// and small enough that one runaway job cannot fill the database on its own.
//
// The cap is on stored bytes, not on the job. The script keeps running and its exit
// status is still reported truthfully; only the log tail is lost.
const DefaultRunLogMaxBytes = 5 << 20

// logCap tracks how much log each in-flight run has stored.
//
// It lives in memory rather than in a SQL counter because the alternative is an extra
// round trip on the hottest write path in the system. The cost of that choice is that
// a control-plane restart resets the counter mid-run, which at worst lets one run store
// a second budget's worth. That is an acceptable trade for not touching the database
// twice per log line.
type logCap struct {
	max int

	mu   sync.Mutex
	used map[string]int
	told map[string]bool // runs already given the truncation notice
}

func newLogCap(max int) *logCap {
	if max <= 0 {
		max = DefaultRunLogMaxBytes
	}
	return &logCap{max: max, used: map[string]int{}, told: map[string]bool{}}
}

// admit records n more bytes for a run and reports whether they should be stored.
// notice is true exactly once per run, on the chunk that crosses the limit, so the
// caller can write a single "output truncated" line into the log.
func (c *logCap) admit(runID string, n int) (store bool, notice bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.used[runID] >= c.max {
		if !c.told[runID] {
			c.told[runID] = true
			return false, true
		}
		return false, false
	}
	c.used[runID] += n
	return true, false
}

// done forgets a finished run. Without this the maps grow for the life of the process.
func (c *logCap) done(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.used, runID)
	delete(c.told, runID)
}
