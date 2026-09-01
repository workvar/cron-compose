package connectors

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// listenSock is one kernel listen socket before we know which connector owns it.
type listenSock struct {
	Proto   string
	Address string
	Port    int
	PID     int
	Process string
}

// portOwner is the connector object a PID belongs to (a systemd unit or a pm2 id).
type portOwner struct {
	Ref  string
	Name string
}

// ListenPort is one row in the Ports tab.
type ListenPort struct {
	Proto     string `json:"proto"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	Process   string `json:"process"`
	Ref       string `json:"ref"`
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

var ssUserRe = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

func parseSsListen(raw string) []listenSock {
	var out []listenSock
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "LISTEN" {
			continue
		}
		host, port := splitListenAddr(fields[3])
		if port <= 0 {
			continue
		}
		m := ssUserRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, _ := strconv.Atoi(m[2])
		if pid <= 0 {
			continue
		}
		out = append(out, listenSock{
			Proto:   "tcp",
			Address: host,
			Port:    port,
			PID:     pid,
			Process: m[1],
		})
	}
	return out
}

func parseLsofListen(raw string) []listenSock {
	var out []listenSock
	for _, line := range strings.Split(raw, "\n") {
		if !strings.Contains(line, "(LISTEN)") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 {
			continue
		}
		host, port := splitListenAddr(fields[len(fields)-2])
		if port <= 0 {
			continue
		}
		out = append(out, listenSock{
			Proto:   "tcp",
			Address: host,
			Port:    port,
			PID:     pid,
			Process: fields[0],
		})
	}
	return out
}

func splitListenAddr(s string) (string, int) {
	if strings.HasPrefix(s, "[") {
		i := strings.LastIndex(s, "]:")
		if i < 0 {
			return "", 0
		}
		port, err := strconv.Atoi(s[i+2:])
		if err != nil {
			return "", 0
		}
		return s[:i+1], port
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return "", 0
	}
	return s[:i], port
}

func systemdUnitFromCgroup(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path := line
		if i := strings.LastIndex(line, ":"); i >= 0 {
			path = line[i+1:]
		}
		parts := strings.Split(path, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			p := parts[i]
			if strings.HasSuffix(p, ".service") || strings.HasSuffix(p, ".socket") {
				return p
			}
		}
	}
	return ""
}

func portIsProtected(s listenSock, unit string, selfPID int) bool {
	if s.PID <= 1 || (selfPID > 0 && s.PID == selfPID) {
		return true
	}
	name := strings.ToLower(s.Process)
	if strings.Contains(name, "croncompose") {
		return true
	}
	u := strings.ToLower(unit)
	return strings.HasPrefix(u, "croncompose-agent")
}

func attachOwners(socks []listenSock, owners map[int]portOwner, selfPID int) []ListenPort {
	var out []ListenPort
	for _, s := range socks {
		o, ok := owners[s.PID]
		if !ok {
			continue
		}
		out = append(out, ListenPort{
			Proto:     s.Proto,
			Address:   s.Address,
			Port:      s.Port,
			PID:       s.PID,
			Process:   s.Process,
			Ref:       o.Ref,
			Name:      o.Name,
			Protected: portIsProtected(s, o.Ref, selfPID),
		})
		if len(out) >= 250 {
			break
		}
	}
	return out
}

func listeningSockets(ctx context.Context) []listenSock {
	if has("ss") {
		out, err := run(ctx, "ss", "-H", "-lntp")
		if err != nil || out == "" {
			out, err = run(ctx, "ss", "-lntp")
		}
		if err == nil {
			if socks := parseSsListen(out); len(socks) > 0 {
				return socks
			}
		}
	}
	if has("lsof") {
		out, err := run(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
		if err == nil {
			return parseLsofListen(out)
		}
	}
	return nil
}

func unitForPID(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cgroup")
	if err != nil {
		return ""
	}
	return systemdUnitFromCgroup(string(b))
}

func portsResult(ports []ListenPort) Result {
	if ports == nil {
		ports = []ListenPort{}
	}
	body, err := json.Marshal(ports)
	if err != nil {
		return fail(StatusFailed, "could not encode ports: "+err.Error())
	}
	n := strconv.Itoa(len(ports))
	return Result{
		Status:  StatusSucceeded,
		Message: n + " listening ports",
		Payload: body,
		Steps:   []Step{step("ports", true, n+" sockets")},
	}
}
