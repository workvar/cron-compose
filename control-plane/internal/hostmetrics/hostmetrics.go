// Package hostmetrics samples CPU, memory, and disk usage for the control-plane host.
package hostmetrics

import (
	"runtime"
	"time"
)

// Snapshot is a point-in-time reading of the machine running the control plane.
type Snapshot struct {
	Hostname      string    `json:"hostname"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	CPUs          int       `json:"cpus"`
	UptimeSec     int64     `json:"uptime_sec,omitempty"`
	Load1         float64   `json:"load1,omitempty"`
	Load5         float64   `json:"load5,omitempty"`
	Load15        float64   `json:"load15,omitempty"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemTotalBytes uint64    `json:"mem_total_bytes"`
	MemUsedBytes  uint64    `json:"mem_used_bytes"`
	MemPercent    float64   `json:"mem_percent"`
	DiskTotalBytes uint64   `json:"disk_total_bytes"`
	DiskUsedBytes  uint64   `json:"disk_used_bytes"`
	DiskPercent    float64  `json:"disk_percent"`
	CollectedAt   time.Time `json:"collected_at"`
}

// Collect reads host stats. Best-effort: partial data is returned rather than failing
// the whole snapshot when one probe is unavailable.
func Collect() Snapshot {
	s := Snapshot{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUs:        runtime.NumCPU(),
		CollectedAt: time.Now().UTC(),
	}
	fillPlatform(&s)
	if s.MemTotalBytes > 0 {
		s.MemPercent = pct(s.MemUsedBytes, s.MemTotalBytes)
	}
	if s.DiskTotalBytes > 0 {
		s.DiskPercent = pct(s.DiskUsedBytes, s.DiskTotalBytes)
	}
	return s
}

func pct(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
