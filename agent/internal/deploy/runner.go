package deploy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Coarse stages, attached to every event so the UI can group a run's output and an
// operator reading raw logs can see where a failure happened.
const (
	phasePreflight = "preflight"
	phaseClone     = "clone"
	phaseInstall   = "install"
	phaseRelease   = "release"
	phaseStart     = "start"
	phaseHealth    = "health"
)

// Bounds on a single run. The timeout keeps a wedged installer (one waiting on a
// prompt nobody will answer, say) from holding the project forever; the log cap
// keeps a runaway build from filling the database with megabytes of progress bars.
//
// Default is 60m (not 15m): a Next.js + Go install on a Pi routinely exceeds a
// quarter-hour even when healthy. Projects can still set deploy_timeout lower.
const (
	defaultRunTimeout = 60 * time.Minute
	maxRunTimeout     = 2 * time.Hour
	logCapBytes       = 2 << 20 // 2MB of streamed output per run
)

// phaseError tags a failure with the stage it happened in. The stage travels to the
// control plane on the finished event, which is what lets a notification say the app
// built fine and then failed its health check, rather than just "failed".
type phaseError struct {
	phase string
	err   error
}

func (e phaseError) Error() string { return e.err.Error() }
func (e phaseError) Unwrap() error { return e.err }

func failedPhase(err error) string {
	var pe phaseError
	if errors.As(err, &pe) {
		return pe.phase
	}
	return ""
}

// Sender ships one DeployEvent back to the control plane. Callers should use the
// durable outbox path so progress survives reconnects (unlike terminal I/O).
type Sender func(*agentv1.DeployEvent)

// Manager owns in-flight deploys, keyed by run id, so stdin can be forwarded into the
// installer PTY and cancel can kill the process.
type Manager struct {
	log  *slog.Logger
	send Sender

	mu       sync.Mutex
	stdin    map[string]*os.File
	cancel   map[string]context.CancelFunc
	seq      map[string]*atomic.Int32
	logged   map[string]*atomic.Int64 // bytes of log streamed per run, for the cap
	capNoted map[string]bool          // whether the "truncated" notice was already sent
}

// NewManager wires a Manager.
func NewManager(log *slog.Logger, send Sender) *Manager {
	return &Manager{
		log:      log,
		send:     send,
		stdin:    map[string]*os.File{},
		cancel:   map[string]context.CancelFunc{},
		seq:      map[string]*atomic.Int32{},
		logged:   map[string]*atomic.Int64{},
		capNoted: map[string]bool{},
	}
}

// Handle routes one inbound DeployCommand. start/cancel run on a goroutine so the
// receive loop is never blocked by git or an interactive installer.
//
// The stream context is not the deploy's parent. That context dies when the
// connection cycle ends, and a stalled stream (a Next.js build pegging a small
// host) was canceling installs that had already compiled. Progress is queued in
// the outbox and flushes when the stream returns. The run stops on its own
// timeout, an explicit cancel, or CloseAll during process shutdown.
func (m *Manager) Handle(_ context.Context, cmd *agentv1.DeployCommand) {
	switch cmd.GetOp() {
	case "stdin":
		m.writeStdin(cmd.GetRunId(), cmd.GetStdin())
	case "cancel":
		m.cancelRun(cmd.GetRunId())
	default:
		go m.start(cmd)
	}
}

// CloseAll cancels every in-flight deploy. The runtime calls this when the agent
// process is shutting down, not when the control-plane stream drops.
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

// runTimeout clamps the control plane's budget into something sane. A project can ask
// for longer than the default for a genuinely slow build, but not for unbounded.
func runTimeout(seconds int32) time.Duration {
	if seconds <= 0 {
		return defaultRunTimeout
	}
	d := time.Duration(seconds) * time.Second
	if d > maxRunTimeout {
		return maxRunTimeout
	}
	return d
}

func (m *Manager) start(cmd *agentv1.DeployCommand) {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	budget := runTimeout(cmd.GetTimeoutSeconds())
	// Detached from the gRPC stream. See Handle.
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	m.mu.Lock()
	m.cancel[runID] = cancel
	m.seq[runID] = &atomic.Int32{}
	m.logged[runID] = &atomic.Int64{}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.cancel, runID)
		delete(m.seq, runID)
		delete(m.logged, runID)
		delete(m.capNoted, runID)
		if f := m.stdin[runID]; f != nil {
			_ = f.Close()
			delete(m.stdin, runID)
		}
		m.mu.Unlock()
		cancel()
	}()

	m.emit(runID, token, &agentv1.DeployEvent{RunId: runID, Kind: "started", Status: "running", Message: "starting"})

	if err := m.deploy(ctx, cmd); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			m.fail(runID, token, phaseInstall, fmt.Sprintf("deploy timed out after %s", budget))
			return
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			m.fail(runID, token, failedPhase(err), "deploy canceled")
			return
		}
		m.fail(runID, token, failedPhase(err), redact(err.Error(), token))
		return
	}
	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "finished", Status: "succeeded", Message: "deploy finished",
	})
}

// deploy walks one run through its phases and returns the first failure. It is split
// out of start so every exit path gets the same timeout and failure reporting.
//
// Pipeline:
//  1. clone the full repo into a disposable tmp directory
//  2. cherry-pick each app's root folder into <clone_path>/releases/…
//  3. build (install script) inside that release folder
//  4. cleanup non-runtime files
//  5. activate release, start process manager, persist boot config
//  6. delete the tmp clone
func (m *Manager) deploy(ctx context.Context, cmd *agentv1.DeployCommand) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()

	base := filepath.Clean(cmd.GetDestPath())
	if base == "" || base == "." || !filepath.IsAbs(base) {
		return errors.New("dest_path must be an absolute path")
	}
	branch := cmd.GetBranch()
	if branch == "" {
		branch = "main"
	}

	cred, err := osuser.Resolve(cmd.GetRunAsUser())
	if err != nil {
		return phaseError{phasePreflight, err}
	}
	tmpDir := userTmpDir(cred)

	if err := m.preflight(runID, token, base, cmd.GetInstallScript(), cmd.GetProcessManager(), cred); err != nil {
		return phaseError{phasePreflight, err}
	}

	layout := NewLayout(base)
	if layout.NeedsMigration() {
		m.phaseLine(runID, token, phaseRelease, "migrating existing checkout into "+releasesDir+"/")
		moved, err := layout.Migrate(ctx)
		if err != nil {
			return phaseError{phaseRelease, fmt.Errorf("migrate to release layout: %w", err)}
		}
		_ = chownTree(moved, cred)
		m.phaseLine(runID, token, phaseRelease, "existing checkout is now "+filepath.Base(moved))
	}

	// Rollback to a release already on disk: no network, no tmp clone.
	if pin := cmd.GetRollbackSha(); pin != "" {
		if dir := layout.FindRelease(pin); dir != "" {
			m.phaseLine(runID, token, phaseRelease, "rolling back to release "+filepath.Base(dir)+" already on disk")
			m.emit(runID, token, &agentv1.DeployEvent{
				RunId: runID, Kind: "log", Phase: phaseClone, CommitSha: pin,
				Data: []byte("commit: " + pin + "\n"),
			})
			if err := layout.Swap(dir); err != nil {
				return phaseError{phaseRelease, fmt.Errorf("activate release: %w", err)}
			}
			m.phaseLine(runID, token, phaseRelease, "current -> "+filepath.Base(dir))
			if err := m.startApps(ctx, cmd, layout, cred, tmpDir); err != nil {
				return phaseError{phaseStart, err}
			}
			if err := m.persistProcessManagers(ctx, cmd, layout, cred, tmpDir); err != nil {
				return phaseError{phaseStart, err}
			}
			if err := m.checkHealth(ctx, cmd); err != nil {
				return phaseError{phaseHealth, err}
			}
			return nil
		}
		m.phaseLine(runID, token, phaseClone, "commit "+shortSHA(pin)+" is not on disk; fetching it")
	}

	cloneDir, sha, err := m.cloneToTmp(ctx, cmd, branch, cred, tmpDir)
	if err != nil {
		return phaseError{phaseClone, err}
	}
	defer func() {
		m.phaseLine(runID, token, phaseClone, "removing tmp clone "+cloneDir)
		_ = os.RemoveAll(cloneDir)
	}()

	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "log", Phase: phaseClone, CommitSha: sha,
		Data: []byte("commit: " + sha + "\n"),
	})

	release, err := m.materializeRelease(ctx, cmd, layout, cloneDir, sha, cred)
	if err != nil {
		return phaseError{phaseClone, err}
	}
	_ = writeSpec(release, cmd)
	_ = chownTree(release, cred)

	if err := m.installApps(ctx, cmd, release, cred, tmpDir); err != nil {
		return phaseError{phaseInstall, err}
	}
	if err := m.cleanupApps(ctx, cmd, release, cred, tmpDir); err != nil {
		return phaseError{phaseInstall, err}
	}

	// Nothing serves from the new release until this point, which is what makes a
	// deploy atomic: a failed install leaves the previous release live.
	if err := layout.Swap(release); err != nil {
		return phaseError{phaseRelease, fmt.Errorf("activate release: %w", err)}
	}
	_ = chownTree(base, cred)
	m.phaseLine(runID, token, phaseRelease, "current -> "+filepath.Base(release))

	if err := m.startApps(ctx, cmd, layout, cred, tmpDir); err != nil {
		return phaseError{phaseStart, err}
	}
	if err := m.persistProcessManagers(ctx, cmd, layout, cred, tmpDir); err != nil {
		return phaseError{phaseStart, err}
	}
	if err := m.checkHealth(ctx, cmd); err != nil {
		return phaseError{phaseHealth, err}
	}
	if pruned := layout.Prune(); len(pruned) > 0 {
		m.phaseLine(runID, token, phaseRelease, "pruned old releases: "+strings.Join(pruned, ", "))
	}
	return nil
}

// cloneToTmp clones the full repository into a disposable directory under the
// deploy user's tmp. The release tree is built by cherry-picking app roots from
// this clone; the clone itself is deleted after the run.
func (m *Manager) cloneToTmp(ctx context.Context, cmd *agentv1.DeployCommand, branch string, cred *osuser.Credential, tmpDir string) (cloneDir, sha string, err error) {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", "", err
	}
	_ = chownTree(tmpDir, cred)
	cloneDir = filepath.Join(tmpDir, "croncompose-clone-"+sanitizeSegment(runID))
	_ = os.RemoveAll(cloneDir)
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		return "", "", err
	}
	_ = chownTree(cloneDir, cred)

	provider := providerFromURL(cmd.GetCloneUrl())
	m.phaseLine(runID, token, phaseClone, "cloning "+PublicCloneURL(cmd.GetCloneUrl())+" ("+branch+") into tmp")
	if err := cloneInto(ctx, cloneDir, cmd.GetCloneUrl(), provider, token, branch, cmd.GetRollbackSha(), cred, tmpDir); err != nil {
		_ = os.RemoveAll(cloneDir)
		return "", "", err
	}
	sha = headCommit(ctx, cloneDir, cred, tmpDir)
	m.phaseLine(runID, token, phaseClone, "tmp clone ready ("+shortSHA(sha)+")")
	return cloneDir, sha, nil
}

// materializeRelease creates the release directory and copies only each app's
// root folder from the tmp clone into it (no sibling monorepo packages).
func (m *Manager) materializeRelease(_ context.Context, cmd *agentv1.DeployCommand, layout Layout, cloneDir, sha string, cred *osuser.Credential) (string, error) {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()

	staging, err := layout.Prepare(runID)
	if err != nil {
		return "", err
	}
	_ = chownTree(staging, cred)

	apps := appsOf(cmd, "")
	multi := len(apps) > 1
	for _, app := range apps {
		root := appRoot(cmd, app)
		src := cloneDir
		if root != "." {
			src = filepath.Join(cloneDir, root)
		}
		dest := staging
		label := "release root"
		if multi {
			sub := releaseSubdir(app, root)
			dest = filepath.Join(staging, sub)
			label = sub
		}
		m.phaseLine(runID, token, phaseClone, "cherry-pick "+root+" -> "+label)
		if err := copyAppRoot(src, dest); err != nil {
			_ = os.RemoveAll(staging)
			return "", err
		}
	}

	release, err := layout.Promote(staging, sha)
	if err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	_ = chownTree(release, cred)
	m.phaseLine(runID, token, phaseClone, "release "+filepath.Base(release))
	return release, nil
}

// installApps runs each app's install script inside the new release directory.
func (m *Manager) installApps(ctx context.Context, cmd *agentv1.DeployCommand, release string, cred *osuser.Credential, tmpDir string) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	for _, app := range appsOf(cmd, release) {
		work := appWorkDir(release, cmd, app)
		script := app.GetInstallScript()
		if script == "" {
			script = cmd.GetInstallScript()
		}
		if script == "" {
			continue
		}
		m.phaseLine(runID, token, phaseInstall, script+" (in "+work+")")
		if err := m.runPTY(ctx, runID, token, work, script, appEnv(cmd, app), cred, tmpDir); err != nil {
			return fmt.Errorf("%s: %w", script, err)
		}
	}
	return nil
}

// cleanupApps runs each app's cleanup script after a successful build so the
// release keeps runtime artifacts and drops source / caches.
func (m *Manager) cleanupApps(ctx context.Context, cmd *agentv1.DeployCommand, release string, cred *osuser.Credential, tmpDir string) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	for _, app := range appsOf(cmd, release) {
		script := cleanupScriptFor(app.GetCleanupScript(), app.GetLanguage())
		if script == "" {
			continue
		}
		work := appWorkDir(release, cmd, app)
		m.phaseLine(runID, token, phaseInstall, "cleanup: "+script+" (in "+work+")")
		if err := m.runPTY(ctx, runID, token, work, script, appEnv(cmd, app), cred, tmpDir); err != nil {
			return fmt.Errorf("cleanup: %w", err)
		}
	}
	return nil
}

// startApps hands each app to its process manager. Working directories go through the
// current symlink, never a release directory, so a pm2 or systemd entry written today
// still points at live code after the next deploy.
func (m *Manager) startApps(ctx context.Context, cmd *agentv1.DeployCommand, layout Layout, cred *osuser.Credential, tmpDir string) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	for _, app := range appsOf(cmd, layout.Current) {
		work := appWorkDir(layout.Current, cmd, app)
		pm := app.GetProcessManager()
		if pm == "" {
			pm = cmd.GetProcessManager()
		}
		name := app.GetName()
		if name == "" {
			name = filepath.Base(work)
		}
		if err := m.startProcess(ctx, runID, token, work, pm, name, app.GetLanguage(), app.GetRunScript(), appEnv(cmd, app), cred, tmpDir); err != nil {
			return err
		}
	}
	return nil
}

// persistProcessManagers runs the boot-persistence step once per distinct process
// manager used by the project (pm2 save + startup, etc.).
func (m *Manager) persistProcessManagers(ctx context.Context, cmd *agentv1.DeployCommand, layout Layout, cred *osuser.Credential, tmpDir string) error {
	seen := map[string]bool{}
	for _, app := range appsOf(cmd, layout.Current) {
		pm := app.GetProcessManager()
		if pm == "" {
			pm = cmd.GetProcessManager()
		}
		if pm == "" || pm == "none" || seen[pm] {
			continue
		}
		seen[pm] = true
		work := appWorkDir(layout.Current, cmd, app)
		if err := m.persistProcessManager(ctx, cmd.GetRunId(), cmd.GetCloneToken(), work, pm, cred, tmpDir); err != nil {
			return err
		}
	}
	return nil
}

// checkHealth runs each app's probe against its own port: the app's own health
// check when it set one, otherwise the project's shared check. This lets a monorepo
// give each process a different path (a Go API's /healthz is not a Next.js
// frontend's) while a project that never set per-app checks keeps behaving exactly
// as before.
func (m *Manager) checkHealth(ctx context.Context, cmd *agentv1.DeployCommand) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	checked := false
	anyConfigured := cmd.GetHealth().GetPath() != ""
	for _, app := range appsOf(cmd, "") {
		port := app.GetPort()
		if port == 0 {
			port = cmd.GetPort()
		}
		hc := cmd.GetHealth()
		if app.GetHealth().GetPath() != "" {
			hc = app.GetHealth()
			anyConfigured = true
		}
		cfg, ok := resolveHealth(hc, port)
		if !ok {
			continue
		}
		checked = true
		if err := m.waitHealthy(ctx, runID, token, cfg); err != nil {
			return err
		}
	}
	if !checked && anyConfigured {
		m.phaseLine(runID, token, phaseHealth, "skipped: no port configured to probe")
	}
	return nil
}

// appsOf returns the command's apps, or one synthesized app for a single-app project.
// dir is used only to guess the language and name; pass "" when neither matters.
func appsOf(cmd *agentv1.DeployCommand, dir string) []*agentv1.DeployApp {
	if apps := cmd.GetApps(); len(apps) > 0 {
		return apps
	}
	name, lang := "app", ""
	if dir != "" {
		name, lang = filepath.Base(dir), detectLangHint(dir)
	}
	return []*agentv1.DeployApp{{
		Name:           name,
		RootDirectory:  cmd.GetRootDirectory(),
		InstallScript:  cmd.GetInstallScript(),
		Language:       lang,
		Port:           cmd.GetPort(),
		ProcessManager: cmd.GetProcessManager(),
		Env:            cmd.GetEnv(),
	}}
}

func appRoot(cmd *agentv1.DeployCommand, app *agentv1.DeployApp) string {
	root := app.GetRootDirectory()
	if root == "" {
		root = cmd.GetRootDirectory()
	}
	return normalizeRoot(root)
}

// releaseSubdir is where a multi-app cherry-pick lands inside the release.
func releaseSubdir(app *agentv1.DeployApp, root string) string {
	if root != "." {
		return root
	}
	sub := sanitizeSegment(app.GetName())
	if sub == "" || sub == "run" {
		return "app"
	}
	return sub
}

// appWorkDir is the directory install/start/cleanup run in. After cherry-pick,
// a single-app release is flattened to the release root (the repo root_directory
// is only the source filter). Multi-app releases keep each app under its subdir.
func appWorkDir(release string, cmd *agentv1.DeployCommand, app *agentv1.DeployApp) string {
	apps := appsOf(cmd, "")
	if len(apps) <= 1 {
		return release
	}
	return filepath.Join(release, releaseSubdir(app, appRoot(cmd, app)))
}

func appEnv(cmd *agentv1.DeployCommand, app *agentv1.DeployApp) map[string]string {
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
	return env
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

// cloneInto clones into an empty directory. Releases are immutable, so there is no
// pull path any more: every deploy gets a fresh shallow clone, and a rollback to a
// commit that is no longer on disk asks the remote for that sha specifically (a sha
// reachable from an advertised ref, which GitHub, GitLab and a stock git server all
// allow).
func cloneInto(ctx context.Context, dest, cloneURL, provider, token, branch, pinSHA string, cred *osuser.Credential, tmpDir string) error {
	authURL := AuthenticatedCloneURL(cloneURL, provider, token)
	public := PublicCloneURL(cloneURL)
	if err := runCmdAs(ctx, "", nil, cred, tmpDir, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", authURL, dest); err != nil {
		return err
	}
	if pinSHA != "" {
		if err := checkoutPinned(ctx, dest, pinSHA, cred, tmpDir); err != nil {
			return err
		}
	}
	// Drop the credentialed remote: the release directory outlives the run, and the
	// token must not sit in .git/config afterwards.
	return runCmdAs(ctx, dest, nil, cred, tmpDir, "git", "remote", "set-url", "origin", public)
}

// checkoutPinned fetches one commit by sha (unshallowing first, best-effort, so the
// fetch has real history to walk) and checks it out.
func checkoutPinned(ctx context.Context, dest, sha string, cred *osuser.Credential, tmpDir string) error {
	_ = runCmdAs(ctx, dest, nil, cred, tmpDir, "git", "fetch", "--unshallow", "origin") // no-op if already full
	if err := runCmdAs(ctx, dest, nil, cred, tmpDir, "git", "fetch", "origin", sha); err != nil {
		return err
	}
	return runCmdAs(ctx, dest, nil, cred, tmpDir, "git", "checkout", "--force", sha)
}

// headCommit returns the checked-out commit sha, or "" if that can't be determined
// (not fatal: the run just won't be a rollback candidate later).
func headCommit(ctx context.Context, dir string, cred *osuser.Credential, tmpDir string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	cmd.Env = credEnv(cred, tmpDir, nil)
	applyCredential(cmd, cred)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func isGitDir(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && st.IsDir()
}

func (m *Manager) runPTY(ctx context.Context, runID, token, dir, script string, env map[string]string, cred *osuser.Credential, tmpDir string) error {
	// Wrap so nvm/fnm under the deploy user are on PATH (non-interactive bash -lc
	// otherwise skips ~/.bashrc and npm "disappears").
	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", osuser.WrapScript(script))
	cmd.Dir = dir
	// Quiet installer chrome: npm's PTY spinner is ESC[1G ESC[0K spam in a log
	// pane that is not a real terminal emulator. CI/NO_COLOR/progress=false cut
	// most of it at the source; scrubPTYLog strips whatever remains.
	merged := map[string]string{
		"CI":                  "true",
		"NO_COLOR":            "1",
		"FORCE_COLOR":         "0",
		"TERM":                "dumb",
		"npm_config_progress": "false",
	}
	for k, v := range env {
		merged[k] = v
	}
	cmd.Env = credEnv(cred, tmpDir, merged)
	applyCredential(cmd, cred)
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
					RunId: runID, Kind: "log", Phase: phaseInstall, Data: append([]byte(nil), buf[:n]...),
				})
			}
			if err != nil {
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	// Give the PTY reader a beat to flush the installer’s final lines (npm ERR!,
	// stack traces) before the caller emits FAILED — otherwise the UI often shows
	// a bare failure with no trailing output.
	time.Sleep(100 * time.Millisecond)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) {
		return fmt.Errorf("script exited %d", ee.ExitCode())
	}
	return waitErr
}

func runCmd(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
	return runCmdAs(ctx, dir, env, nil, "", name, args...)
}

func runCmdAs(ctx context.Context, dir string, env map[string]string, cred *osuser.Credential, tmpDir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil || cred != nil || tmpDir != "" {
		cmd.Env = credEnv(cred, tmpDir, env)
	}
	applyCredential(cmd, cred)
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

func (m *Manager) fail(runID, token, phase, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = "deploy failed"
	}
	if phase != "" && !strings.HasPrefix(msg, phase+":") {
		msg = phase + ": " + msg
	}
	// Surface the reason in the log stream too — the finished event’s Message is
	// easy to miss when the UI only updates status/exit from the SSE done event.
	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "log", Phase: phase, Data: []byte("FAILED — " + msg + "\n"),
	})
	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "finished", Status: "failed", ExitCode: 1, Phase: phase, Message: msg,
	})
}

func (m *Manager) logLine(runID, token, msg string) {
	m.emit(runID, token, &agentv1.DeployEvent{RunId: runID, Kind: "log", Data: []byte(msg + "\n")})
}

// phaseLine is logLine with a stage attached, for the structured phases above.
func (m *Manager) phaseLine(runID, token, phase, msg string) {
	m.emit(runID, token, &agentv1.DeployEvent{
		RunId: runID, Kind: "log", Phase: phase, Data: []byte(phase + ": " + msg + "\n"),
	})
}

// overLogCap reports whether this run has already streamed its budget of output, and
// sends one notice the first time it has. Lifecycle events are never dropped; only
// log payload is, so a capped run still finishes cleanly.
func (m *Manager) overLogCap(runID string, n int) bool {
	m.mu.Lock()
	counter := m.logged[runID]
	m.mu.Unlock()
	if counter == nil {
		return false
	}
	if counter.Add(int64(n)) <= logCapBytes {
		return false
	}
	m.mu.Lock()
	first := !m.capNoted[runID]
	m.capNoted[runID] = true
	m.mu.Unlock()
	if first && m.send != nil {
		m.send(&agentv1.DeployEvent{
			RunId: runID, Kind: "log", Phase: phaseInstall,
			Data: []byte(fmt.Sprintf("\n[log truncated: this run passed %s of output]\n", humanBytes(logCapBytes))),
		})
	}
	return true
}

func (m *Manager) emit(runID, token string, ev *agentv1.DeployEvent) {
	if ev.Kind == "log" && len(ev.Data) > 0 {
		ev.Data = scrubPTYLog(ev.Data)
		if len(ev.Data) == 0 {
			return
		}
	}
	if ev.Kind == "log" && len(ev.Data) > 0 && m.overLogCap(runID, len(ev.Data)) {
		return
	}
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
