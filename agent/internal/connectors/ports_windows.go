//go:build windows

package connectors

import (
	"context"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// listeningSockets enumerates TCP listen sockets with `netstat -ano`, which every
// Windows edition ships and which reports the owning PID without elevation.
func listeningSockets(ctx context.Context) []listenSock {
	out, err := run(ctx, "netstat", "-ano", "-p", "TCP")
	socks := parseNetstatListen(out)
	if out6, err6 := run(ctx, "netstat", "-ano", "-p", "TCPv6"); err6 == nil {
		socks = append(socks, parseNetstatListen(out6)...)
	}
	if err != nil && len(socks) == 0 {
		return nil
	}
	names := processNames()
	for i := range socks {
		socks[i].Process = names[socks[i].PID]
	}
	return socks
}

// parseNetstatListen reads rows like
//
//	TCP    0.0.0.0:3000     0.0.0.0:0     LISTENING     4242
//	TCP    [::]:3000        [::]:0        LISTENING     4242
func parseNetstatListen(raw string) []listenSock {
	var out []listenSock
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.EqualFold(f[0], "TCP") || f[3] != "LISTENING" {
			continue
		}
		host, port := splitListenAddr(f[1])
		pid, err := strconv.Atoi(f[4])
		if port <= 0 || err != nil || pid <= 0 {
			continue
		}
		key := f[1] + "/" + f[4]
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, listenSock{Proto: "tcp", Address: host, Port: port, PID: pid})
	}
	return out
}

// processEntries snapshots every process once (Toolhelp32), returning pid -> entry.
func processEntries() map[int]windows.ProcessEntry32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	out := map[int]windows.ProcessEntry32{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out[int(e.ProcessID)] = e
	}
	return out
}

func processNames() map[int]string {
	names := map[int]string{}
	for pid, e := range processEntries() {
		names[pid] = strings.TrimSuffix(windows.UTF16ToString(e.ExeFile[:]), ".exe")
	}
	return names
}

// childLister snapshots the process table once and answers child lookups from it,
// since Windows has no per-process children file to read.
func childLister() func(int) []int {
	kids := map[int][]int{}
	for pid, e := range processEntries() {
		parent := int(e.ParentProcessID)
		if parent != pid {
			kids[parent] = append(kids[parent], pid)
		}
	}
	return func(pid int) []int { return kids[pid] }
}
