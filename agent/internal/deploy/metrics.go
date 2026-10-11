package deploy

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// runMetrics accumulates per-phase timings and sizes for one deploy run.
type runMetrics struct {
	mu            sync.Mutex
	phaseStart    time.Time
	currentPhase  string
	phaseMs       map[string]int64
	releaseBytes  int64
	freeDiskBytes int64
}

func newRunMetrics() *runMetrics {
	return &runMetrics{phaseMs: map[string]int64{}, phaseStart: time.Now()}
}

func (r *runMetrics) notePhase(phase string) {
	if phase == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if r.currentPhase != "" {
		r.phaseMs[r.currentPhase] += now.Sub(r.phaseStart).Milliseconds()
	}
	r.currentPhase = phase
	r.phaseStart = now
}

func (r *runMetrics) setFreeDisk(n int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.freeDiskBytes = n
}

func (r *runMetrics) setReleaseBytes(n int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releaseBytes = n
}

func (r *runMetrics) snapshot() *agentv1.DeployMetrics {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	out := map[string]int64{}
	for k, v := range r.phaseMs {
		out[k] = v
	}
	if r.currentPhase != "" {
		out[r.currentPhase] += now.Sub(r.phaseStart).Milliseconds()
	}
	return &agentv1.DeployMetrics{
		PhaseMs:       out,
		ReleaseBytes:  r.releaseBytes,
		FreeDiskBytes: r.freeDiskBytes,
	}
}

// dirSize walks a tree and returns total file bytes (best-effort).
func dirSize(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
