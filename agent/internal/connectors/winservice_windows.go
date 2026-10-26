//go:build windows

package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/platform"
)

// winServiceProvider is the Windows counterpart of the systemd connector: it lists
// Windows services and drives their lifecycle through the Service Control Manager.
type winServiceProvider struct{}

func (winServiceProvider) Kind() string { return "winservice" }

// winService is one row of Win32_Service.
type winService struct {
	Name        string `json:"Name"`
	DisplayName string `json:"DisplayName"`
	State       string `json:"State"`
	StartMode   string `json:"StartMode"`
	PathName    string `json:"PathName"`
	ProcessID   int    `json:"ProcessId"`
	StartName   string `json:"StartName"`
}

const listServicesPS = `Get-CimInstance Win32_Service | ` +
	`Select-Object Name,DisplayName,State,StartMode,PathName,ProcessId,StartName | ` +
	`ConvertTo-Json -Compress`

func (p *winServiceProvider) Detect(ctx context.Context) []Instance {
	svcs, err := listServices(ctx)
	if err != nil {
		return nil
	}
	manageable := platform.IsElevated()
	return []Instance{{
		Kind:        "winservice",
		Status:      "running",
		ObjectCount: len(svcs),
		Manageable:  manageable,
		Caps: Capabilities{
			ManagesObjects: true,
			CanLifecycle:   manageable,
		},
		Detail: map[string]string{"manager": "Service Control Manager"},
	}}
}

func (p *winServiceProvider) Resources(ctx context.Context, _ Instance) []Resource {
	svcs, err := listServices(ctx)
	if err != nil {
		return nil
	}
	res := make([]Resource, 0, len(svcs))
	for _, s := range svcs {
		res = append(res, Resource{
			Type:  "object",
			Ref:   s.Name,
			Name:  s.DisplayName,
			State: winServiceState(s.State),
			Attributes: map[string]string{
				"start_mode": s.StartMode,
				"exec_start": s.PathName,
				"pid":        strconv.Itoa(s.ProcessID),
				"run_as":     s.StartName,
			},
		})
		if len(res) >= 400 {
			break
		}
	}
	return res
}

// Lifecycle maps the allowlisted verbs onto PowerShell's service cmdlets. The
// service name is validated, then passed as a single-quoted literal.
func (p *winServiceProvider) Lifecycle(ctx context.Context, _ Instance, ref, action string) Result {
	if !ValidAction(action) {
		return fail(StatusUnsupported, "unknown action: "+action)
	}
	if !validServiceName(ref) {
		return fail(StatusFailed, "invalid service name: "+ref)
	}
	cmdlet := map[string]string{
		"start": "Start-Service", "stop": "Stop-Service -Force", "restart": "Restart-Service -Force",
		"pause": "Suspend-Service", "unpause": "Resume-Service",
	}[action]
	if cmdlet == "" {
		return fail(StatusUnsupported, "action not supported for Windows services: "+action)
	}
	script := fmt.Sprintf("%s -Name '%s' -ErrorAction Stop; (Get-Service -Name '%s').Status", cmdlet, ref, ref)
	out, err := powershell(ctx, script)
	s := step(cmdlet+" "+ref, err == nil, trimOutput(out))
	if err != nil {
		return fail(StatusFailed, cmdlet+" failed: "+trimOutput(out), s)
	}
	return ok("service "+ref+" "+action+" done", s)
}

// Logs returns the service's recent Application events plus Service Control
// Manager events that name it, newest first.
func (p *winServiceProvider) Logs(ctx context.Context, _ Instance, ref string, lines int) Result {
	if !validServiceName(ref) {
		return fail(StatusFailed, "invalid service name: "+ref)
	}
	if lines <= 0 || lines > 1000 {
		lines = 200
	}
	script := fmt.Sprintf(`$n='%s'; $d=(Get-Service -Name $n -ErrorAction SilentlyContinue).DisplayName
$a=@(Get-WinEvent -FilterHashtable @{LogName='Application';ProviderName=$n} -MaxEvents %d -ErrorAction SilentlyContinue)
$s=@(Get-WinEvent -FilterHashtable @{LogName='System';ProviderName='Service Control Manager'} -MaxEvents 500 -ErrorAction SilentlyContinue | Where-Object { $_.Message -like "*$d*" } | Select-Object -First %d)
($a+$s) | Sort-Object TimeCreated -Descending | ForEach-Object { '{0:u} [{1}] {2}' -f $_.TimeCreated,$_.LevelDisplayName,($_.Message -replace '\s+',' ') }`,
		ref, lines, lines)
	out, err := powershell(ctx, script)
	s := step("Get-WinEvent "+ref, err == nil, trimOutput(out))
	if err != nil {
		return fail(StatusFailed, "reading event log failed: "+trimOutput(out), s)
	}
	return Result{Status: StatusSucceeded, Message: "event log " + ref, Content: []byte(out), Steps: []Step{s}}
}

// Ports lists listening sockets owned by a running service (or anything it started).
// svchost-hosted services share a PID, so their sockets are attributed to the
// first service in that process, which is the best Windows can say.
func (p *winServiceProvider) Ports(ctx context.Context, _ Instance) Result {
	svcs, err := listServices(ctx)
	if err != nil {
		return fail(StatusFailed, "listing services failed: "+err.Error())
	}
	children := childLister()
	owners := map[int]portOwner{}
	for _, s := range svcs {
		if s.ProcessID <= 0 {
			continue
		}
		o := portOwner{Ref: s.Name, Name: s.DisplayName}
		queue := []int{s.ProcessID}
		for len(queue) > 0 && len(owners) < 5000 {
			pid := queue[0]
			queue = queue[1:]
			if _, seen := owners[pid]; seen {
				continue
			}
			owners[pid] = o
			queue = append(queue, children(pid)...)
		}
	}
	return portsResult(attachOwners(listeningSockets(ctx), owners, os.Getpid()))
}

func listServices(ctx context.Context) ([]winService, error) {
	out, err := powershell(ctx, listServicesPS)
	if err != nil {
		return nil, err
	}
	return parseWinServices(out)
}

// parseWinServices accepts ConvertTo-Json output, which is an array for many rows
// and a bare object for exactly one.
func parseWinServices(raw string) ([]winService, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "{") {
		raw = "[" + raw + "]"
	}
	var out []winService
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func winServiceState(s string) string {
	switch s {
	case "Running":
		return "running"
	case "Paused":
		return "paused"
	default:
		return "stopped"
	}
}

// validServiceName rejects anything that could escape the single-quoted literal
// the name is placed in. '$' stays legal (SQL Server instances use it) because it
// is inert inside single quotes.
func validServiceName(n string) bool {
	if n == "" || len(n) > 256 {
		return false
	}
	return !strings.ContainsAny(n, "'\"`;|&<>/\\\r\n\t")
}

func powershell(ctx context.Context, script string) (string, error) {
	return run(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
}

func platformProviders() []Provider { return []Provider{&winServiceProvider{}} }
