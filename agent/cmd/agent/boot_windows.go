//go:build windows

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/platform"
)

// maxLogBytes rotates the service log once at startup when it grows past this.
const maxLogBytes = 10 << 20

// fileLogger writes to <DATA_DIR>\logs\agent.log, keeping one previous file.
func fileLogger(dataDir string) (*slog.Logger, func()) {
	dir := filepath.Join(dataDir, "logs")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "agent.log")
	if st, err := os.Stat(path); err == nil && st.Size() > maxLogBytes {
		_ = os.Rename(path, path+".1")
	}
	var w io.Writer = os.Stderr
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		w = f
	}
	closer := func() {
		if f != nil {
			_ = f.Close()
		}
	}
	return slog.New(slog.NewTextHandler(w, nil)), closer
}

// bootHooks prepares the process environment for Windows children and tidies up
// after a self-update:
//   - Git Bash settings every child inherits (see platform.ExtraEnv).
//   - The previous binary (<exe>.old) can be deleted once the new one is running.
//   - pm2 has no Windows boot integration (`pm2 startup` is Linux/macOS only), so
//     the agent resurrects the saved pm2 process list when it starts.
func bootHooks(ctx context.Context, log *slog.Logger) {
	for _, kv := range platform.ExtraEnv() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			_ = os.Setenv(k, v)
		}
	}
	if exe, err := os.Executable(); err == nil {
		if err := os.Remove(exe + ".old"); err == nil {
			log.Info("removed previous agent binary", "path", exe+".old")
		}
	}
	go resurrectPM2(ctx, log)
}

func resurrectPM2(ctx context.Context, log *slog.Logger) {
	if strings.EqualFold(os.Getenv("AGENT_PM2_RESURRECT"), "0") {
		return
	}
	pm2, err := platform.LookPath("pm2")
	if err != nil || !fileExists(pm2DumpFile()) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, pm2, "resurrect").CombinedOutput()
	if err != nil {
		log.Warn("pm2 resurrect failed", "err", err, "output", strings.TrimSpace(string(out)))
		return
	}
	log.Info("pm2 process list resurrected")
}

func pm2DumpFile() string {
	home := os.Getenv("PM2_HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(h, ".pm2")
		}
	}
	return filepath.Join(home, "dump.pm2")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
