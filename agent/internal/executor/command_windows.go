//go:build windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	"github.com/croncompose/croncompose/agent/internal/platform"
	"github.com/croncompose/croncompose/agent/internal/procgroup"
)

// defaultPath is the PATH of last resort on Windows.
const defaultPath = `C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0`

// groupHandle lets Cancel reach the Job Object that attach creates after Start.
type groupHandle struct {
	mu sync.Mutex
	g  *procgroup.Group
}

func (h *groupHandle) set(g *procgroup.Group) {
	h.mu.Lock()
	h.g = g
	h.mu.Unlock()
}

func (h *groupHandle) kill() {
	h.mu.Lock()
	g := h.g
	h.mu.Unlock()
	if g != nil {
		_ = g.Kill()
	}
}

// prepare builds the command for one run on Windows. bash/sh scripts run under Git
// Bash so the same script works on every platform; powershell, pwsh, and cmd are
// native options. Scripts for the native shells go through a temp file because
// multi-line bodies do not survive being squeezed into one command-line argument.
func prepare(ctx context.Context, interp string, j Job, _ *osuser.Credential) (*prepared, error) {
	p := &prepared{cleanup: func() {}}
	prog, args, file, err := windowsCommand(interp, j.ScriptBody)
	if err != nil {
		return nil, err
	}
	if file != "" {
		p.cleanup = func() { _ = os.Remove(file) }
	}
	p.cmd = exec.CommandContext(ctx, prog, args...)
	if strings.EqualFold(interp, "cmd") {
		// cmd.exe parses its own command line; let it see the quotes exactly.
		p.cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /D /S /C ""` + file + `""`}
	}
	cmd := p.cmd
	cmd.Cancel = func() error {
		p.group.kill()
		return cmd.Process.Kill()
	}
	// A grandchild that inherited stdout must not hold Wait open forever.
	cmd.WaitDelay = 10 * time.Second
	if j.IOWeight > 0 {
		p.notes = append(p.notes, "io_weight is not supported on Windows; ignoring it")
	}
	return p, nil
}

// attach moves the started interpreter into a Job Object carrying the job's caps,
// so timeout and cancel stop the whole tree and limits cover every child.
func (p *prepared) attach(j Job) (func(), []string) {
	lim := procgroup.Limits{CPUQuotaPercent: j.CPUQuotaPercent, MemoryMaxMB: j.MemoryMaxMB, TasksMax: j.TasksMax}
	g, err := procgroup.Attach(p.cmd.Process, lim)
	if err != nil {
		var notes []string
		if lim.Any() {
			notes = append(notes, "resource limits requested but could not be applied ("+err.Error()+"); running unlimited")
		}
		return func() {}, notes
	}
	p.group.set(g)
	return func() { _ = g.Close() }, nil
}

// windowsCommand maps an interpreter name to (program, args, tempFile).
func windowsCommand(interp, script string) (string, []string, string, error) {
	switch strings.ToLower(interp) {
	case "bash", "sh":
		if !platform.HasBash() {
			return "", nil, "", fmt.Errorf("no bash found for a %s job: %s", interp, platform.BashHint)
		}
		return platform.Bash(), []string{"-c", script}, "", nil
	case "powershell", "pwsh":
		file, err := writeScript(script, ".ps1")
		if err != nil {
			return "", nil, "", err
		}
		prog := "powershell.exe"
		if strings.EqualFold(interp, "pwsh") {
			prog = "pwsh.exe"
		}
		return prog, []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", file}, file, nil
	case "cmd":
		file, err := writeScript("@echo off\r\n"+script, ".cmd")
		if err != nil {
			return "", nil, "", err
		}
		return "cmd.exe", nil, file, nil
	case "python3", "python":
		return pythonBinary(), []string{"-c", script}, "", nil
	case "node":
		return "node", []string{"-e", script}, "", nil
	default:
		return interp, []string{"-c", script}, "", nil
	}
}

// pythonBinary prefers python.exe: on Windows "python3" is often only the Microsoft
// Store stub, which opens the Store instead of running anything.
func pythonBinary() string {
	for _, name := range []string{"python.exe", "python3.exe", "py.exe"} {
		if p, err := exec.LookPath(name); err == nil && !strings.Contains(strings.ToLower(p), `\windowsapps\`) {
			return p
		}
	}
	return "python.exe"
}

func writeScript(body, ext string) (string, error) {
	f, err := os.CreateTemp("", "croncompose-job-*"+ext)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
