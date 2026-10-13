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
		name := QualifyProcessName(project, appName)
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
	Pm2Env struct {
		Status string `json:"status"`
		Cwd    string `json:"pm_cwd"`
	} `json:"pm2_env"`
}

// waitPM2Online polls `pm2 jlist` until the process we just started is online.
// Ecosystem files choose their own process name, so a name match falls back to
// the working directory. The returned name is what log tailing should follow.
func (m *Manager) waitPM2Online(ctx context.Context, runID, token, phase, work, want string, cred *osuser.Credential, tmpDir string) (string, error) {
	deadline := time.Now().Add(readyBudget)
	m.phaseLine(runID, token, phase, "waiting for pm2 process "+want+" (in "+work+")")
	var last string
	for {
		out, err := runCmdOut(ctx, work, nil, cred, tmpDir, "pm2", "jlist")
		if err != nil {
			return "", fmt.Errorf("pm2 jlist: %w", err)
		}
		proc, ok := pickPM2(parsePM2Jlist(out), want, work)
		if ok {
			status := strings.ToLower(strings.TrimSpace(proc.Pm2Env.Status))
			last = proc.Name + "=" + status
			switch status {
			case "online":
				m.phaseLine(runID, token, phase, "pm2 online: "+proc.Name+" (in "+work+")")
				if proc.Name == "" {
					return want, nil
				}
				return proc.Name, nil
			case "errored", "stopped", "stopping":
				return "", fmt.Errorf("pm2 process %s is %s", proc.Name, status)
			}
		}
		if time.Now().After(deadline) {
			if last == "" {
				last = "not in pm2 jlist"
			}
			return "", fmt.Errorf("pm2 process %s did not come online within %s (%s)", want, readyBudget, last)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
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

// pickPM2 finds the process pm2 actually registered. A qualified name wins;
// otherwise the single process whose cwd is the app directory (ecosystem files
// name themselves).
func pickPM2(procs []pm2Listed, want, work string) (pm2Listed, bool) {
	work = filepath.Clean(work)
	base := filepath.Base(work)
	var cwdHits []pm2Listed
	for _, p := range procs {
		if p.Name != "" && (p.Name == want || p.Name == base) {
			return p, true
		}
		if filepath.Clean(p.Pm2Env.Cwd) == work {
			cwdHits = append(cwdHits, p)
		}
	}
	for _, p := range cwdHits {
		if strings.EqualFold(p.Pm2Env.Status, "online") {
			return p, true
		}
	}
	if len(cwdHits) > 0 {
		return cwdHits[0], true
	}
	return pm2Listed{}, false
}
