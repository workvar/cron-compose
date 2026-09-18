//go:build darwin

package hostmetrics

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

func fillPlatform(s *Snapshot) {
	if h, err := os.Hostname(); err == nil {
		s.Hostname = h
	}
	fillLoad(s)
	fillMem(s)
	fillDisk(s, "/")
	// Darwin lacks a cheap /proc/stat sample; approximate from load vs CPUs.
	if s.CPUs > 0 && s.Load1 > 0 {
		p := s.Load1 / float64(s.CPUs) * 100
		if p > 100 {
			p = 100
		}
		s.CPUPercent = p
	}
}

func fillLoad(s *Snapshot) {
	b, err := unix.SysctlRaw("vm.loadavg")
	if err != nil || len(b) < 24 {
		return
	}
	type loadavg struct {
		Ldavg [3]uint32
		_     uint32
		Scale uint64
	}
	la := (*loadavg)(unsafe.Pointer(&b[0]))
	if la.Scale == 0 {
		return
	}
	scale := float64(la.Scale)
	s.Load1 = float64(la.Ldavg[0]) / scale
	s.Load5 = float64(la.Ldavg[1]) / scale
	s.Load15 = float64(la.Ldavg[2]) / scale
}

func fillMem(s *Snapshot) {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return
	}
	s.MemTotalBytes = total

	pageSize := uint64(unix.Getpagesize())
	freePages, _ := unix.SysctlUint64("vm.page_free_count")
	inactive, _ := unix.SysctlUint64("vm.page_inactive_count")
	speculative, _ := unix.SysctlUint64("vm.page_speculative_count")
	available := (freePages + inactive + speculative) * pageSize
	if total >= available {
		s.MemUsedBytes = total - available
	}
}

func fillDisk(s *Snapshot, path string) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	free := st.Bavail * bsize
	s.DiskTotalBytes = total
	if total >= free {
		s.DiskUsedBytes = total - free
	}
}
