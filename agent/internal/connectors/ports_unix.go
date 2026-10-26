//go:build !windows

package connectors

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// listeningSockets enumerates TCP listen sockets on the host. Unprivileged `ss -p`
// often omits process columns, so we fall back to lsof and then privileged retries
// when the agent has a sudo grant for ss or lsof.
func listeningSockets(ctx context.Context) []listenSock {
	if socks := ssListen(ctx, false); len(socks) > 0 {
		return socks
	}
	if socks := lsofListen(ctx, false); len(socks) > 0 {
		return socks
	}
	if socks := ssListen(ctx, true); len(socks) > 0 {
		return socks
	}
	return lsofListen(ctx, true)
}

func readChildPIDs(pid int) []int {
	path := filepath.Join("/proc", strconv.Itoa(pid), "task", strconv.Itoa(pid), "children")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []int
	for _, field := range strings.Fields(string(b)) {
		child, err := strconv.Atoi(field)
		if err == nil && child > 1 {
			out = append(out, child)
		}
	}
	return out
}

// childLister returns a lookup of a PID's direct children, read live from /proc.
func childLister() func(int) []int { return readChildPIDs }
