package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

const readyBudget = 90 * time.Second

// runApps executes the Run phase: launch each app and do not return until it is
// accepting connections (or, with no port, until it has stayed up). The next
// flow wave waits on this, which is how Proxy can be running before Backend
// is even installed.
func (m *Manager) runApps(ctx context.Context, cmd *agentv1.DeployCommand, layout Layout, cred *osuser.Credential, tmpDir string, apps []*agentv1.DeployApp) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	project := cmd.GetProjectName()
	var errs []error
	for _, app := range apps {
		work := appWorkDir(layout.Current, cmd, app)
		appName := app.GetName()
		if appName == "" {
			appName = filepath.Base(work)
		}
		pm := app.GetProcessManager()
		if pm == "" {
			pm = cmd.GetProcessManager()
		}
		name := ResolveHostProcessName(project, app, work)
		port := app.GetPort()
		if port == 0 {
			port = cmd.GetPort()
		}
		err := m.startProcess(ctx, runID, token, work, pm, name, app.GetLanguage(), app.GetRunScript(), appEnv(cmd, app), cred, tmpDir, phaseRun, port)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", appName, err))
			continue
		}
		m.noteLaunched(runID, appName)
	}
	return joinErrors(errs)
}

// runDirect starts the run script itself and leaves it running after the phase
// is done. A process manager is not required: this is the "run the code" step.
func (m *Manager) runDirect(ctx context.Context, runID, token, work, language, runScript string, env map[string]string, cred *osuser.Credential, tmpDir, phase string, port int32) error {
	bin, args := startCommand(language, runScript, filepath.Base(work))
	if bin == "" {
		m.phaseLine(runID, token, phase, "no run command (in "+work+")")
		return fmt.Errorf("no run command for %s", work)
	}
	shown := strings.TrimSpace(runScript)
	if shown == "" {
		shown = bin
		if len(args) > 0 {
			shown += " " + strings.Join(args, " ")
		}
	}
	m.phaseLine(runID, token, phase, shown+" (in "+work+")")

	resolved := resolveBin(cred, bin)
	// Not CommandContext: a successful deploy must not kill the app it just started.
	cmd := exec.Command(resolved, args...)
	cmd.Dir = work
	cmd.Env = credEnv(cred, tmpDir, pathWithBinDir(env, resolved))
	applyCredential(cmd, cred)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devNull.Close()
	cmd.Stdin = devNull
	if err := cmd.Start(); err != nil {
		return err
	}
	m.trackProc(runID, cmd.Process)
	go m.pumpOutput(runID, token, work, stdout, true)

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	if err := m.waitReadyExit(ctx, runID, token, phase, work, port, exited); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return err
	}
	m.phaseLine(runID, token, phase, "done (in "+work+")")
	return nil
}

// runCmdStream runs a command and forwards its combined output as it arrives,
// instead of holding every byte until the process exits.
func (m *Manager) runCmdStream(ctx context.Context, runID, token, dir string, env map[string]string, cred *osuser.Credential, tmpDir, name string, args ...string) error {
	resolved := resolveBin(cred, name)
	cmd := exec.CommandContext(ctx, resolved, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	merged := pathWithBinDir(env, resolved)
	if merged != nil || cred != nil || tmpDir != "" {
		cmd.Env = credEnv(cred, tmpDir, merged)
	}
	applyCredential(cmd, cred)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.pumpOutput(runID, token, dir, stdout, dir != "")
	}()
	waitErr := cmd.Wait()
	<-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) {
		return fmt.Errorf("%s exited %d", name, ee.ExitCode())
	}
	return fmt.Errorf("%s: %w", name, waitErr)
}

// followLogs tails the supervisor's log for the rest of the deploy so the run
// board keeps showing application output after pm2/systemd/docker has daemonized.
func (m *Manager) followLogs(ctx context.Context, runID, token, work, pm, name string, cred *osuser.Credential, tmpDir string) {
	var bin string
	var args []string
	switch pm {
	case "pm2":
		bin, args = "pm2", []string{"logs", name, "--raw", "--lines", "50"}
	case "systemd":
		bin, args = "journalctl", []string{"--user", "-fu", systemdUnitName(name), "-n", "20", "--no-pager"}
	case "docker":
		bin, args = "docker", []string{"logs", "-f", "--tail", "20", name}
	default:
		return
	}
	_ = m.runCmdStream(ctx, runID, token, work, nil, cred, tmpDir, bin, args...)
}

func (m *Manager) waitReady(ctx context.Context, runID, token, phase, work string, port int32) error {
	return m.waitReadyExit(ctx, runID, token, phase, work, port, nil)
}

func (m *Manager) waitReadyExit(ctx context.Context, runID, token, phase, work string, port int32, exited <-chan error) error {
	if port <= 0 {
		if exited == nil {
			return nil
		}
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case err := <-exited:
			if err != nil {
				return fmt.Errorf("process exited: %w", err)
			}
			return fmt.Errorf("process exited before it was ready")
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	deadline := time.Now().Add(readyBudget)
	m.phaseLine(runID, token, phase, "waiting for "+addr+" (in "+work+")")
	for {
		if exited != nil {
			select {
			case err := <-exited:
				if err != nil {
					return fmt.Errorf("process exited: %w", err)
				}
				return fmt.Errorf("process exited before %s accepted connections", addr)
			default:
			}
		}
		conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("nothing accepted connections on %s within %s", addr, readyBudget)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// portFrom uses the app port, then PORT from the environment the process is
// started with, so a Run step still waits when the only port is in env.
func portFrom(port int32, env map[string]string) int32 {
	if port > 0 {
		return port
	}
	n, err := strconv.Atoi(strings.TrimSpace(env["PORT"]))
	if err != nil || n <= 0 || n > 65535 {
		return 0
	}
	return int32(n)
}

type pm2Listed struct {
	Name   string `json:"name"`
	Pid    int    `json:"pid"`
	Pm2Env struct {
		Status string `json:"status"`
		Cwd    string `json:"pm_cwd"`
		PmID   int    `json:"pm_id"`
	} `json:"pm2_env"`
}

const pm2JlistTimeout = 4 * time.Second

// waitPM2Online polls `pm2 jlist` until the process we just started is online.
// Ecosystem files choose their own process name, so a name match falls back to
// aliases and the working directory. The returned name is what log tailing follows.
//
// Each jlist call has its own short timeout: a hung `pm2 jlist` must not block
// past readyBudget (that looked like "forever waiting" on the run page).
func (m *Manager) waitPM2Online(ctx context.Context, runID, token, phase, work, want string, aliases []string, cred *osuser.Credential, tmpDir string) (string, error) {
	deadline := time.Now().Add(readyBudget)
	label := want
	if len(aliases) > 0 && aliases[0] != want {
		label = want + " / " + strings.Join(aliases, ", ")
	}
	m.phaseLine(runID, token, phase, "waiting for pm2 process "+label+" (in "+work+")")
	var last string
	candidates := pm2NameCandidates(want, aliases...)
	for {
		if time.Now().After(deadline) {
			break
		}
		procs, jerr := m.pm2Jlist(ctx, work, cred, tmpDir)
		if jerr != nil {
			last = "jlist: " + jerr.Error()
		} else {
			proc, ok := pickPM2(procs, want, work, aliases...)
			if ok {
				status := strings.ToLower(strings.TrimSpace(proc.Pm2Env.Status))
				last = proc.Name + "=" + status
				if pm2IsReady(proc) {
					name := proc.Name
					if name == "" {
						name = want
					}
					m.phaseLine(runID, token, phase, "pm2 online: "+name+" (in "+work+")")
					return name, nil
				}
				switch status {
				case "errored", "stopped", "stopping":
					return "", fmt.Errorf("pm2 process %s is %s", proc.Name, status)
				}
			} else if real, ok := m.pm2PidOnline(ctx, work, cred, tmpDir, candidates...); ok {
				m.phaseLine(runID, token, phase, "pm2 online: "+real+" (via pid; in "+work+")")
				return real, nil
			} else if last == "" {
				names := make([]string, 0, len(procs))
				for _, p := range procs {
					if p.Name != "" {
						names = append(names, p.Name)
					}
				}
				if len(names) == 0 {
					last = "pm2 jlist empty"
				} else {
					last = "jlist has " + strings.Join(names, ", ") + " (wanted " + label + ")"
				}
			}
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	if last == "" {
		last = "pm2 jlist empty — start may have failed (check ecosystem script/cwd)"
	}
	return "", fmt.Errorf("pm2 process %s did not come online within %s (%s)", want, readyBudget, last)
}

func (m *Manager) pm2Jlist(ctx context.Context, work string, cred *osuser.Credential, tmpDir string) ([]pm2Listed, error) {
	jctx, cancel := context.WithTimeout(ctx, pm2JlistTimeout)
	defer cancel()
	out, err := runCmdOut(jctx, work, nil, cred, tmpDir, "pm2", "jlist")
	if err != nil {
		return nil, err
	}
	return parsePM2Jlist(out), nil
}

func pm2IsReady(p pm2Listed) bool {
	status := strings.ToLower(strings.TrimSpace(p.Pm2Env.Status))
	switch status {
	case "online":
		return true
	case "errored", "stopped", "stopping", "one-launch-status":
		return false
	}
	// Some pm2 builds omit status in jlist but still report a live pid.
	return p.Pid > 0 && status != "stopped" && status != "errored"
}

func pm2NameCandidates(want string, aliases ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range append([]string{want}, aliases...) {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// pm2PidOnline is a fallback when jlist is empty/unparseable after a successful
// `pm2 start`: `pm2 pid <name>` returns a positive pid when the process is up.
func (m *Manager) pm2PidOnline(ctx context.Context, work string, cred *osuser.Credential, tmpDir string, names ...string) (string, bool) {
	for _, name := range names {
		if name == "" {
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		out, err := runCmdOut(pctx, work, nil, cred, tmpDir, "pm2", "pid", name)
		cancel()
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil || pid <= 0 {
			continue
		}
		return name, true
	}
	return "", false
}

// pm2AlreadyOnline is a cheap pre-start check so the Start wave after Run does
// not call `pm2 start` again when the process is already up.
func (m *Manager) pm2AlreadyOnline(ctx context.Context, work, want string, aliases []string, cred *osuser.Credential, tmpDir string) (string, bool) {
	procs, err := m.pm2Jlist(ctx, work, cred, tmpDir)
	if err == nil {
		if proc, ok := pickPM2(procs, want, work, aliases...); ok && pm2IsReady(proc) {
			if proc.Name != "" {
				return proc.Name, true
			}
			return want, true
		}
	}
	return m.pm2PidOnline(ctx, work, cred, tmpDir, pm2NameCandidates(want, aliases...)...)
}

func parsePM2Jlist(out string) []pm2Listed {
	for i := 0; i < len(out); i++ {
		if out[i] != '[' {
			continue
		}
		var procs []pm2Listed
		if err := json.Unmarshal([]byte(out[i:]), &procs); err == nil {
			return procs
		}
	}
	return nil
}

// pickPM2 finds the process pm2 actually registered. Ecosystem files choose
// their own name (peepal-proxy) while we register peepal-rp-proxy, and pm2's
// cwd is often the release path while work is …/current/proxy — Resolve those.
func pickPM2(procs []pm2Listed, want, work string, aliases ...string) (pm2Listed, bool) {
	work = resolvePath(work)
	base := filepath.Base(work)
	wants := make([]string, 0, 1+len(aliases))
	wants = append(wants, want)
	wants = append(wants, aliases...)
	var nameHits, cwdHits []pm2Listed
	for _, p := range procs {
		if p.Name == "" {
			continue
		}
		hit := false
		for _, w := range wants {
			if w != "" && (p.Name == w || pm2NameMatch(p.Name, w, base)) {
				hit = true
				break
			}
		}
		if hit {
			nameHits = append(nameHits, p)
			continue
		}
		if sameWorkDir(p.Pm2Env.Cwd, work, base) {
			cwdHits = append(cwdHits, p)
		}
	}
	if hit, ok := preferOnline(nameHits); ok {
		return hit, true
	}
	if hit, ok := preferOnline(cwdHits); ok {
		return hit, true
	}
	return pm2Listed{}, false
}

func preferOnline(procs []pm2Listed) (pm2Listed, bool) {
	for _, p := range procs {
		if strings.EqualFold(p.Pm2Env.Status, "online") {
			return p, true
		}
	}
	if len(procs) > 0 {
		return procs[0], true
	}
	return pm2Listed{}, false
}

func resolvePath(p string) string {
	p = filepath.Clean(p)
	if p == "" {
		return p
	}
	if real, err := filepath.EvalSymlinks(p); err == nil && real != "" {
		return real
	}
	return p
}

func sameWorkDir(cwd, work, base string) bool {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return false
	}
	c := resolvePath(cwd)
	w := resolvePath(work)
	if c == w {
		return true
	}
	// Release dir vs current/ symlink, or trailing segment only.
	if base != "" && (strings.HasSuffix(c, string(filepath.Separator)+base) || filepath.Base(c) == base) {
		// Prefer a hit under the same project tree when both paths share a parent.
		if w == "" {
			return true
		}
		wp := filepath.Dir(w)
		cp := filepath.Dir(c)
		if wp == cp {
			return true
		}
		// …/releases/<id>/proxy vs …/current/proxy after EvalSymlinks failed on one side.
		if strings.Contains(c, string(filepath.Separator)+"releases"+string(filepath.Separator)) &&
			strings.Contains(w, string(filepath.Separator)+"current") &&
			filepath.Base(c) == base && filepath.Base(w) == base {
			return true
		}
		if strings.Contains(w, string(filepath.Separator)+"releases"+string(filepath.Separator)) &&
			strings.Contains(c, string(filepath.Separator)+"current") &&
			filepath.Base(c) == base && filepath.Base(w) == base {
			return true
		}
	}
	return false
}

// pm2NameMatch: peepal-rp-proxy ↔ peepal-proxy ↔ proxy.
func pm2NameMatch(got, want, appBase string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" {
		return false
	}
	if got == want || got == appBase {
		return true
	}
	if appBase != "" && (got == appBase || strings.HasSuffix(got, "-"+appBase)) {
		return true
	}
	if want != "" && strings.HasSuffix(want, "-"+got) {
		return true
	}
	// peepal-rp-proxy vs peepal-proxy: same trailing segment.
	if appBase != "" && strings.HasSuffix(want, "-"+appBase) && strings.HasSuffix(got, "-"+appBase) {
		return true
	}
	return false
}
