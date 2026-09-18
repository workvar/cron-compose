//go:build linux

package hostmetrics

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func fillPlatform(s *Snapshot) {
	if h, err := os.Hostname(); err == nil {
		s.Hostname = h
	}
	fillLoad(s)
	fillMem(s)
	fillDisk(s, "/")
	s.CPUPercent = sampleCPU(120 * time.Millisecond)
	if u := uptimeSec(); u > 0 {
		s.UptimeSec = u
	}
}

func fillLoad(s *Snapshot) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	parts := strings.Fields(string(b))
	if len(parts) < 3 {
		return
	}
	s.Load1, _ = strconv.ParseFloat(parts[0], 64)
	s.Load5, _ = strconv.ParseFloat(parts[1], 64)
	s.Load15, _ = strconv.ParseFloat(parts[2], 64)
}

func fillMem(s *Snapshot) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	var total, available uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemKB(line) * 1024
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemKB(line) * 1024
		}
	}
	s.MemTotalBytes = total
	if total >= available {
		s.MemUsedBytes = total - available
	}
}

func parseMemKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, _ := strconv.ParseUint(fields[1], 10, 64)
	return n
}

func fillDisk(s *Snapshot, path string) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	s.DiskTotalBytes = total
	if total >= free {
		s.DiskUsedBytes = total - free
	}
}

type cpuSample struct{ idle, total uint64 }

func readCPU() (cpuSample, bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, false
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	if !strings.HasPrefix(line, "cpu ") {
		return cpuSample{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return cpuSample{}, false
	}
	var nums []uint64
	for _, f := range fields[1:] {
		n, _ := strconv.ParseUint(f, 10, 64)
		nums = append(nums, n)
	}
	var total uint64
	for _, n := range nums {
		total += n
	}
	idle := nums[3]
	if len(nums) > 4 {
		idle += nums[4] // iowait
	}
	return cpuSample{idle: idle, total: total}, true
}

func sampleCPU(wait time.Duration) float64 {
	a, ok := readCPU()
	if !ok {
		return 0
	}
	time.Sleep(wait)
	b, ok := readCPU()
	if !ok || b.total <= a.total {
		return 0
	}
	totalDelta := float64(b.total - a.total)
	idleDelta := float64(b.idle - a.idle)
	used := (totalDelta - idleDelta) / totalDelta * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func uptimeSec() int64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return 0
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(f)
}
