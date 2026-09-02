package connectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func systemdDisplayName(unit string) string {
	name := strings.TrimSuffix(unit, ".service")
	return strings.TrimSuffix(name, ".socket")
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
			unit := unitForPID(s.PID)
			if unit == "" {
				continue
			}
			o = portOwner{Ref: unit, Name: systemdDisplayName(unit)}
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

func ssListen(ctx context.Context, privileged bool) []listenSock {
	if !has("ss") {
		return nil
	}
	var out string
	var err error
	if privileged {
		out, _, err = privRun(ctx, "ss", "-H", "-lntp")
		if err != nil || out == "" {
			out, _, err = privRun(ctx, "ss", "-lntp")
		}
	} else {
		out, err = run(ctx, "ss", "-H", "-lntp")
		if err != nil || out == "" {
			out, err = run(ctx, "ss", "-lntp")
		}
	}
	if err != nil {
		return nil
	}
	return parseSsListen(out)
}

func lsofListen(ctx context.Context, privileged bool) []listenSock {
	if !has("lsof") {
		return nil
	}
	var out string
	var err error
	if privileged {
		out, _, err = privRun(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
	} else {
		out, err = run(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
	}
	if err != nil {
		return nil
	}
	return parseLsofListen(out)
}

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

func unitForPID(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cgroup")
	if err != nil {
		return ""
	}
	return systemdUnitFromCgroup(string(b))
}

func pidsInCgroup(controlGroup string) []int {
	if controlGroup == "" || controlGroup == "/" {
		return nil
	}
	rel := strings.TrimPrefix(controlGroup, "/")
	paths := []string{
		filepath.Join("/sys/fs/cgroup", rel, "cgroup.procs"),
	}
	if rel != "" {
		paths = append(paths, filepath.Join("/sys/fs/cgroup/unified", rel, "cgroup.procs"))
	}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var pids []int
		for _, line := range strings.Split(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil && pid > 1 {
				pids = append(pids, pid)
			}
		}
		if len(pids) > 0 {
			return pids
		}
	}
	return nil
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

// descendantPIDs returns root and every descendant process, best-effort.
func descendantPIDs(root int) []int {
	if root <= 1 {
		return nil
	}
	seen := map[int]bool{root: true}
	queue := []int{root}
	var out []int
	for len(queue) > 0 && len(out) < 500 {
		pid := queue[0]
		queue = queue[1:]
		out = append(out, pid)
		for _, child := range readChildPIDs(pid) {
			if !seen[child] {
				seen[child] = true
				queue = append(queue, child)
			}
		}
	}
	return out
}

func parseSystemctlShowOwners(raw string) map[int]portOwner {
	owners := map[int]portOwner{}
	var block []string
	flush := func() {
		if len(block) == 0 {
			return
		}
		props := map[string]string{}
		for _, line := range block {
			k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
			if ok {
				props[k] = strings.TrimSpace(v)
			}
		}
		block = nil
		unit := props["Unit"]
		if unit == "" {
			return
		}
		o := portOwner{Ref: unit, Name: systemdDisplayName(unit)}
		for _, key := range []string{"MainPID", "ControlPID", "ExecMainPID"} {
			pid, _ := strconv.Atoi(props[key])
			if pid > 1 {
				owners[pid] = o
			}
		}
		for _, pid := range pidsInCgroup(props["ControlGroup"]) {
			owners[pid] = o
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		block = append(block, line)
	}
	flush()
	return owners
}

func systemdOwnerMap(ctx context.Context) map[int]portOwner {
	if !systemdAvailable() {
		return map[int]portOwner{}
	}
	out, err := run(ctx, "systemctl", "show",
		"--type=service", "--type=socket",
		"-p", "MainPID", "-p", "ControlPID", "-p", "ExecMainPID",
		"-p", "Unit", "-p", "ControlGroup")
	if err != nil {
		return map[int]portOwner{}
	}
	return parseSystemctlShowOwners(out)
}

func pm2OwnerMap(procs []pm2Proc) map[int]portOwner {
	owners := map[int]portOwner{}
	for _, pr := range procs {
		if pr.Pid <= 0 {
			continue
		}
		name := pr.Name
		if name == "" {
			name = strconv.Itoa(pr.PmID)
		}
		o := portOwner{Ref: strconv.Itoa(pr.PmID), Name: name}
		for _, pid := range descendantPIDs(pr.Pid) {
			owners[pid] = o
		}
	}
	return owners
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
