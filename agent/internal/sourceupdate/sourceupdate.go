// Package sourceupdate rebuilds the agent (and, on a control-plane host, the whole
// stack) from a GitHub tag instead of downloading a prebuilt binary.
package sourceupdate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/selfupdate"
)

// IsSource reports whether this update instruction means "clone/build from git"
// rather than "download a binary". A notes-only GitHub release is a source update:
// https://github.com/owner/repo with no checksum.
func IsSource(downloadURL, sha256 string) bool {
	return strings.TrimSpace(sha256) == "" && repoFromGitHubURL(downloadURL) != ""
}

func repoFromGitHubURL(raw string) string {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ".git")
	const prefix = "https://github.com/"
	if !strings.HasPrefix(raw, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(raw, prefix)
	if strings.Contains(rest, "/releases/") {
		return ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// FindStackRoot returns the CronCompose git checkout this process is running from,
// or "" if this is a standalone agent install.
func FindStackRoot() string {
	for _, key := range []string{"CC_SOURCE_ROOT", "CRONCOMPOSE_ROOT"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if isStackRoot(v) || dirHasGit(v) {
				return v
			}
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return findStackRootAt(wd)
}

func dirHasGit(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func findStackRootAt(start string) string {
	dir := start
	for i := 0; i < 8; i++ {
		if isStackRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func isStackRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	for _, name := range []string{"ecosystem.config.js", "update.sh", "croncompose-ctl.sh"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// Result tells the caller whether it should exit so a supervisor restarts the process.
type Result struct {
	RestartNow bool
	Path       string
}

// Apply checks out targetVersion from the GitHub repo in downloadURL and builds.
// On a control-plane checkout it starts update.sh (which restarts the stack).
// On a standalone agent it builds a replacement binary and swaps it in place.
func Apply(ctx context.Context, log *slog.Logger, downloadURL, targetVersion, currentVersion string) (Result, error) {
	repo := repoFromGitHubURL(downloadURL)
	if repo == "" {
		return Result{}, fmt.Errorf("not a github source url: %s", downloadURL)
	}
	if targetVersion == "" {
		return Result{}, fmt.Errorf("no target version")
	}
	if targetVersion == currentVersion {
		return Result{}, fmt.Errorf("already running %s", currentVersion)
	}

	if root := FindStackRoot(); root != "" {
		log.Info("source update: rebuilding stack", "root", root, "tag", targetVersion)
		if err := applyStack(ctx, log, root, targetVersion); err != nil {
			return Result{}, err
		}
		return Result{RestartNow: false, Path: root}, nil
	}

	log.Info("source update: rebuilding agent", "repo", repo, "tag", targetVersion)
	path, err := applyAgent(ctx, log, repo, targetVersion)
	if err != nil {
		return Result{}, err
	}
	return Result{RestartNow: true, Path: path}, nil
}

func applyStack(ctx context.Context, log *slog.Logger, root, tag string) error {
	if err := run(ctx, root, "git", "fetch", "--tags", "--force", "origin"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	if err := run(ctx, root, "git", "checkout", "--force", tag); err != nil {
		return fmt.Errorf("git checkout %s: %w", tag, err)
	}
	if err := run(ctx, root, "git", "reset", "--hard", tag); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}

	update := filepath.Join(root, "update.sh")
	if _, err := os.Stat(update); err != nil {
		return fmt.Errorf("update.sh missing after checkout: %w", err)
	}

	logDir := filepath.Join(root, ".run")
	_ = os.MkdirAll(logDir, 0o755)
	logFile, err := os.OpenFile(filepath.Join(logDir, "update.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logFile, err = os.CreateTemp("", "croncompose-update-*.log")
		if err != nil {
			return fmt.Errorf("open update log: %w", err)
		}
	}
	cmd := exec.Command("bash", update, "--no-pull")
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start update.sh: %w", err)
	}
	log.Info("update.sh started", "pid", cmd.Process.Pid, "log", logFile.Name())
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()
	return nil
}

func applyAgent(ctx context.Context, log *slog.Logger, repo, tag string) (string, error) {
	src, err := os.MkdirTemp("", "croncompose-src-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(src)

	url := "https://github.com/" + repo
	if err := run(ctx, "", "git", "clone", "--depth", "1", "--branch", tag, url, src); err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}

	out := filepath.Join(src, "agent", "bin", "agent")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	ver := strings.TrimPrefix(tag, "v")
	ldflags := fmt.Sprintf("-s -w -X github.com/croncompose/croncompose/agent/internal/config.buildVersion=%s", ver)
	if err := run(ctx, filepath.Join(src, "agent"), "go", "build",
		"-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/agent"); err != nil {
		return "", fmt.Errorf("go build: %w", err)
	}

	installed, err := selfupdate.InstallFile(out)
	if err != nil {
		return "", err
	}
	log.Info("agent binary replaced", "path", installed)
	return installed, nil
}

func run(ctx context.Context, dir string, name string, args ...string) error {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 2048 {
			msg = msg[len(msg)-2048:]
		}
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}