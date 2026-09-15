package deploy

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/creack/pty"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Sender ships one DeployEvent back to the control plane (ephemeral, never outbox).
type Sender func(*agentv1.DeployEvent)

// Manager owns in-flight deploys, keyed by run id, so stdin can be forwarded into the
// installer PTY and cancel can kill the process.
type Manager struct {
	log  *slog.Logger
	send Sender

	mu     sync.Mutex
	stdin  map[string]*os.File
	cancel map[string]context.CancelFunc
	seq    map[string]*atomic.Int32
}

// NewManager wires a Manager.
func NewManager(log *slog.Logger, send Sender) *Manager {
	return &Manager{
		log:    log,
		send:   send,
		stdin:  map[string]*os.File{},
		cancel: map[string]context.CancelFunc{},
		seq:    map[string]*atomic.Int32{},
	}
}

// Handle routes one inbound DeployCommand. start/cancel run on a goroutine so the
// receive loop is never blocked by git or an interactive installer.
func (m *Manager) Handle(ctx context.Context, cmd *agentv1.DeployCommand) {
	switch cmd.GetOp() {
	case "stdin":
		m.writeStdin(cmd.GetRunId(), cmd.GetStdin())
	case "cancel":
		m.cancelRun(cmd.GetRunId())
	default:
		go m.start(ctx, cmd)
	}
}

// CloseAll tears down every in-flight deploy (stream dropped).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.cancel))
	for _, c := range m.cancel {
		cancels = append(cancels, c)
	}
	m.cancel = map[string]context.CancelFunc{}
	files := make([]*os.File, 0, len(m.stdin))
	for _, f := range m.stdin {
		files = append(files, f)
	}
	m.stdin = map[string]*os.File{}
	m.mu.Unlock()
	for _, c := range cancels {
		c()
	}
	for _, f := range files {
		_ = f.Close()
	}
}

func (m *Manager) writeStdin(runID string, data []byte) {
	m.mu.Lock()
	f := m.stdin[runID]
	m.mu.Unlock()
	if f != nil && len(data) > 0 {
		_, _ = f.Write(data)
	}
}

func (m *Manager) cancelRun(runID string) {
	m.mu.Lock()
	c := m.cancel[runID]
	m.mu.Unlock()
	if c != nil {
		c()
	}
}

func (m *Manager) start(parent context.Context, cmd *agentv1.DeployCommand) {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	ctx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	m.cancel[runID] = cancel
	m.seq[runID] = &atomic.Int32{}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.cancel, runID)
		delete(m.seq, runID)
		if f := m.stdin[runID]; f != nil {
			_ = f.Close()
			delete(m.stdin, runID)
		}
		m.mu.Unlock()
		cancel()
	}()

	m.emit(runID, token, &agentv1.DeployEvent{RunId: runID, Kind: "started", Status: "running", Message: "starting"})

	dest := filepath.Clean(cmd.GetDestPath())
	if dest == "" || dest == "." || !filepath.IsAbs(dest) {
		m.fail(runID, token, "dest_path must be an absolute path")
		return
	}
	branch := cmd.GetBranch()
	if branch == "" {
		branch = "main"
	}
	// The control plane pins a rollback deploy to an exact commit via this env var
	// rather than a new DeployCommand field, so an older agent build still supports
	// forward deploys unchanged and a newer one needs no proto change to add this.
	// See control-plane/internal/deploys/handler.go's rollbackShaEnvKey.
	pinSHA := ""
	if env := cmd.GetEnv(); env != nil {
		if sha, ok := env[rollbackEnvKey]; ok {
			pinSHA = sha
			cleaned := make(map[string]string, len(env))
			for k, v := range env {
				if k != rollbackEnvKey {
					cleaned[k] = v
				}
			}
			cmd.Env = cleaned
		}
	}
	provider := providerFromURL(cmd.GetCloneUrl())
	if err := cloneOrPull(ctx, dest, cmd.GetCloneUrl(), provider, token, branch, pinSHA); err != nil {
		m.fail(runID, token, redact(err.Error(), token))
		return
	}
	sha := headCommit(ctx, dest)
	m.logLine(runID, token, "cloned "+PublicCloneURL(cmd.GetCloneUrl())+" → "+dest)
	if sha != "" {
		// Parsed back out of the log by control-plane/internal/agentgw/stream.go and
		// stored on the run, so a later failure knows what commit to roll back to.
		m.logLine(runID, token, "commit: "+sha)
	}
	_ = writeSpec(dest, cmd)

	apps := cmd.GetApps()
	if len(apps) == 0 {
		apps = []*agentv1.DeployApp{{
			Name:           filepath.Base(dest),
			RootDirectory:  cmd.GetRootDirectory(),
			InstallScript:  cmd.GetInstallScript(),
			Language:       detectLangHint(dest),
			Port:           cmd.GetPort(),
			ProcessManager: cmd.GetProcessManager(),
			Env:            cmd.GetEnv(),
		}}
	}
	for _, app := range apps {
		root := app.GetRootDirectory()
		if root == "" {
			root = cmd.GetRootDirectory()
		}
		work := dest
		if root != "" && root != "." {
			work = filepath.Join(dest, filepath.Clean(root))
		}
		script := app.GetInstallScript()
		if script == "" {
			script = cmd.GetInstallScript()
		}
		env := map[string]string{}
		for k, v := range cmd.GetEnv() {
			env[k] = v
		}
		for k, v := range app.GetEnv() {
			env[k] = v
		}
		port := app.GetPort()
		if port == 0 {
			port = cmd.GetPort()
		}
		if port > 0 {
			env["PORT"] = fmt.Sprintf("%d", port)
		}
		if script != "" {
			m.logLine(runID, token, "install: "+script+" (in "+work+")")
			if err := m.runPTY(ctx, runID, token, work, script, env); err != nil {
				m.fail(runID, token, redact(err.Error(), token))
				return
			}
		}
		pm := app.GetProcessManager()
		if pm == "" {
			pm = cmd.GetProcessManager()
		}
		lang := app.GetLanguage()
		if err := m.startProcess(ctx, runID, token, work, pm, lang, env); err != nil {
			m.fail(runID, token, redact(err.Error(), token))
			return
		}
	}
	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "finished", Status: "succeeded", Message: "deploy finished",
	})
}

func findEcosystem(dir string) string {
	for _, name := range []string{
		"ecosystem.config.js", "ecosystem.config.cjs", "ecosystem.config.mjs",
		"ecosystem.config.ts", "pm2.json",
	} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// rollbackEnvKey is the DeployCommand.Env key the control plane sets to pin a run to
// an exact commit instead of the branch tip. Must match
// control-plane/internal/deploys/handler.go's rollbackShaEnvKey.
const rollbackEnvKey = "CRONCOMPOSE_ROLLBACK_SHA"

// cloneOrPull clones fresh or fast-forwards an existing checkout to branch's tip. When
// pinSHA is set (an automatic rollback), it checks out that exact commit instead: the
// normal clone is shallow (--depth 1), which only holds the tip, so a rollback fetches
// that commit specifically (asking the remote for a SHA that is reachable from a ref
// it already advertises, which GitHub, GitLab and a stock git server all allow) rather
// than assuming --depth 1 left old history lying around.
func cloneOrPull(ctx context.Context, dest, cloneURL, provider, token, branch, pinSHA string) error {
	authURL := AuthenticatedCloneURL(cloneURL, provider, token)
	public := PublicCloneURL(cloneURL)
	if isGitDir(dest) {
		if err := runCmd(ctx, dest, nil, "git", "remote", "set-url", "origin", authURL); err != nil {
			return err
		}
		defer func() { _ = runCmd(context.Background(), dest, nil, "git", "remote", "set-url", "origin", public) }()
		if err := runCmd(ctx, dest, nil, "git", "fetch", "origin", branch); err != nil {
			return err
		}
		if pinSHA != "" {
			return checkoutPinned(ctx, dest, pinSHA)
		}
		if err := runCmd(ctx, dest, nil, "git", "checkout", branch); err != nil {
			return err
		}
		return runCmd(ctx, dest, nil, "git", "reset", "--hard", "origin/"+branch)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := runCmd(ctx, "", nil, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", authURL, dest); err != nil {
		return err
	}
	if pinSHA == "" {
		return runCmd(ctx, dest, nil, "git", "remote", "set-url", "origin", public)
	}
	defer func() { _ = runCmd(context.Background(), dest, nil, "git", "remote", "set-url", "origin", public) }()
	return checkoutPinned(ctx, dest, pinSHA)
}

// checkoutPinned fetches one commit by sha (unshallowing first, best-effort, so the
// fetch has real history to walk) and checks it out. dest's origin must already carry
// authenticated credentials when the repo is private.
func checkoutPinned(ctx context.Context, dest, sha string) error {
	_ = runCmd(ctx, dest, nil, "git", "fetch", "--unshallow", "origin") // no-op if already full
	if err := runCmd(ctx, dest, nil, "git", "fetch", "origin", sha); err != nil {
		return err
	}
	return runCmd(ctx, dest, nil, "git", "checkout", "--force", sha)
}

// headCommit returns the checked-out commit sha, or "" if that can't be determined
// (not fatal: the run just won't be a rollback candidate later).
func headCommit(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isGitDir(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && st.IsDir()
}

func (m *Manager) runPTY(ctx context.Context, runID, token, dir, script string, env map[string]string) error {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), envPairs(env)...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.stdin[runID] = ptmx
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if m.stdin[runID] == ptmx {
			delete(m.stdin, runID)
		}
		m.mu.Unlock()
		_ = ptmx.Close()
	}()

	go func() {
		r := bufio.NewReader(ptmx)
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				m.emit(runID, token, &agentv1.DeployEvent{
					RunId: runID, Kind: "log", Data: append([]byte(nil), buf[:n]...),
				})
			}
			if err != nil {
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return waitErr
}

func runCmd(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = append(os.Environ(), envPairs(env)...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s: %s", name, msg)
	}
	return nil
}

func envPairs(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func writeSpec(dest string, cmd *agentv1.DeployCommand) error {
	var b strings.Builder
	b.WriteString("provider: auto\n")
	b.WriteString("repo: " + PublicCloneURL(cmd.GetCloneUrl()) + "\n")
	if cmd.GetBranch() != "" {
		b.WriteString("branch: " + cmd.GetBranch() + "\n")
	}
	if cmd.GetInstallScript() != "" {
		b.WriteString("install: " + cmd.GetInstallScript() + "\n")
	}
	if cmd.GetRootDirectory() != "" {
		b.WriteString("root: " + cmd.GetRootDirectory() + "\n")
	}
	if cmd.GetCloneUrl() != "" {
		// dest is the source of truth on this host
		b.WriteString("clone_path: " + dest + "\n")
	}
	if cmd.GetProcessManager() != "" {
		b.WriteString("process_manager: " + cmd.GetProcessManager() + "\n")
	}
	return os.WriteFile(filepath.Join(dest, "croncompose.yml"), []byte(b.String()), 0o644)
}

func (m *Manager) fail(runID, token, msg string) {
	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "finished", Status: "failed", ExitCode: 1, Message: msg,
	})
}

func (m *Manager) logLine(runID, token, msg string) {
	m.emit(runID, token, &agentv1.DeployEvent{RunId: runID, Kind: "log", Data: []byte(msg + "\n")})
}

func (m *Manager) emit(runID, token string, ev *agentv1.DeployEvent) {
	if ev.Seq == 0 {
		m.mu.Lock()
		s := m.seq[runID]
		m.mu.Unlock()
		if s != nil {
			ev.Seq = s.Add(1)
		}
	}
	if ev.Message != "" {
		ev.Message = redact(ev.Message, token)
	}
	if len(ev.Data) > 0 && token != "" {
		ev.Data = []byte(redact(string(ev.Data), token))
	}
	if m.send != nil {
		m.send(ev)
	}
}
