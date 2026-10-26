// Package hosttools detects and installs common deploy toolchains for a chosen
// OS account. Detection and installs run under that user's login shell so nvm,
// asdf, and similar PATH hooks are visible — the same problem deploy preflight
// hits when the agent process PATH is a minimal systemd default.
package hosttools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// LogSink receives live stdout/stderr fragments during install/uninstall.
// percent is a 0–90 hint while the process runs; the final result closes at 100.
type LogSink func(chunk []byte, percent int32)

const (
	detectTimeout  = 30 * time.Second
	installTimeout = 12 * time.Minute
	logCap         = 256 << 10 // 256KB
)

// KnownTools is the catalog the Tools UI shows. Order is display order.
var KnownTools = []string{
	"node", "npm", "pnpm", "yarn", "bun",
	"go", "python", "python3", "pip", "pip3",
	"pm2", "git", "docker", "make",
}

// Installable are tools the agent knows how to install for a given account.
var Installable = map[string]bool{
	"node": true, "go": true, "python": true, "pm2": true,
	"git": true, "yarn": true, "pnpm": true, "bun": true,
}

// Detect reports which known tools are on the chosen user's login PATH.
// When only is non-empty, only that tool (and its aliases) is probed so the UI
// can fill rows progressively instead of waiting for the whole catalog.
func Detect(ctx context.Context, runAs string, only ...string) ([]*agentv1.ToolStatus, error) {
	cred, err := osuser.Resolve(runAs)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()

	names := KnownTools
	if len(only) > 0 && strings.TrimSpace(only[0]) != "" {
		names = expandToolAliases(strings.ToLower(strings.TrimSpace(only[0])))
	}

	out := make([]*agentv1.ToolStatus, 0, len(names))
	for _, name := range names {
		st := &agentv1.ToolStatus{Name: name}
		path, ver, ok := probe(ctx, cred, name)
		st.Installed = ok
		st.Path = path
		st.Version = ver
		out = append(out, st)
	}
	return out, nil
}

func expandToolAliases(tool string) []string {
	switch tool {
	case "python":
		return []string{"python", "python3"}
	case "pip":
		return []string{"pip", "pip3"}
	default:
		return []string{tool}
	}
}

// Install runs a curated installer for tool as runAs and returns a fresh detect
// snapshot plus captured log output. onLog may be nil; when set it receives live
// stdout/stderr chunks as the installer runs.
func Install(ctx context.Context, runAs, tool string, onLog LogSink) (tools []*agentv1.ToolStatus, log string, exitCode int, err error) {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if !Installable[tool] {
		return nil, "", 1, fmt.Errorf("unknown or unsupported tool %q", tool)
	}
	cred, err := osuser.Resolve(runAs)
	if err != nil {
		return nil, "", 1, err
	}
	script, err := installScript(tool, cred)
	if err != nil {
		return nil, "", 1, err
	}
	return runScript(ctx, runAs, cred, script, onLog)
}

// Uninstall removes a curated toolchain install for runAs where we know a safe
// reverse path (user-local installs). Package-manager installs that need root
// are best-effort and may refuse. onLog may be nil.
func Uninstall(ctx context.Context, runAs, tool string, onLog LogSink) (tools []*agentv1.ToolStatus, log string, exitCode int, err error) {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if !Installable[tool] {
		return nil, "", 1, fmt.Errorf("unknown or unsupported tool %q", tool)
	}
	cred, err := osuser.Resolve(runAs)
	if err != nil {
		return nil, "", 1, err
	}
	script, err := uninstallScript(tool, cred)
	if err != nil {
		return nil, "", 1, err
	}
	return runScript(ctx, runAs, cred, script, onLog)
}

func runScript(ctx context.Context, runAs string, cred *osuser.Credential, script string, onLog LogSink) (tools []*agentv1.ToolStatus, log string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	cmd, cleanup, err := scriptCommand(ctx, script)
	if err != nil {
		return nil, "", 1, err
	}
	defer cleanup()
	cmd.Env = buildEnv(cred)
	applyCred(cmd, cred)
	sw := newStreamWriter(onLog)
	cmd.Stdout = sw
	cmd.Stderr = sw
	runErr := cmd.Run()
	log = trimLog(sw.String())
	exitCode = 0
	if runErr != nil {
		exitCode = 1
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		if ctx.Err() != nil {
			return nil, log, exitCode, fmt.Errorf("timed out")
		}
	}

	tools, detErr := Detect(context.Background(), runAs)
	if detErr != nil && runErr == nil {
		return tools, log, exitCode, detErr
	}
	return tools, log, exitCode, runErr
}

// streamWriter tees process output into a capped buffer and optional live sink.
// Percent climbs toward 90% over a soft expected duration so the UI can show
// movement while real log lines arrive.
type streamWriter struct {
	buf    bytes.Buffer
	onLog  LogSink
	start  time.Time
	expect time.Duration
}

func newStreamWriter(onLog LogSink) *streamWriter {
	return &streamWriter{onLog: onLog, start: time.Now(), expect: 3 * time.Minute}
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	// Cap the retained buffer the same way trimLog does at the end.
	if w.buf.Len()+len(p) > logCap*2 {
		keep := w.buf.Bytes()
		if len(keep) > logCap {
			w.buf.Reset()
			_, _ = w.buf.Write(keep[len(keep)-logCap:])
		}
	}
	_, _ = w.buf.Write(p)
	if w.onLog != nil {
		chunk := append([]byte(nil), p...)
		elapsed := time.Since(w.start)
		pct := int32(elapsed * 90 / w.expect)
		if pct < 1 {
			pct = 1
		}
		if pct > 90 {
			pct = 90
		}
		w.onLog(chunk, pct)
	}
	return len(p), nil
}

func (w *streamWriter) String() string { return w.buf.String() }

// Ensure streamWriter satisfies io.Writer.
var _ io.Writer = (*streamWriter)(nil)

func buildEnv(cred *osuser.Credential) []string {
	env := append([]string(nil), os.Environ()...)
	if cred != nil {
		env = append(env, cred.Env()...)
		if cred.Home != "" {
			tmp := filepath.Join(cred.Home, "tmp")
			_ = os.MkdirAll(tmp, 0o755)
			env = append(env, "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
		}
	}
	return env
}

func applyCred(cmd *exec.Cmd, cred *osuser.Credential) { osuser.Apply(cmd, cred) }

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func trimLog(s string) string {
	if len(s) <= logCap {
		return s
	}
	return s[len(s)-logCap:]
}
