package procgroup

// CPURate converts a systemd-style quota (100 = one full core) into a Windows Job
// Object CPU rate, which is in hundredths of a percent of the whole machine. Two
// cores at 100% on an 8-core box is 25% of the machine, or 2500. The result is
// clamped to the range Windows accepts.
func CPURate(quotaPercent, numCPU int) uint32 {
	if numCPU < 1 {
		numCPU = 1
	}
	rate := quotaPercent * 100 / numCPU
	if rate < 1 {
		rate = 1
	}
	if rate > 10000 {
		rate = 10000
	}
	return uint32(rate)
}
