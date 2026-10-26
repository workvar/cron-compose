//go:build !windows

package terminal

import (
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// session is one running terminal: a process attached to a pseudo-terminal. Both modes
// (interactive shell and one-shot command) use the same machinery; they differ only in
// the argv passed to the shell and in whether the process is expected to stay alive.
type session struct {
	id   string
	cmd  *exec.Cmd
	ptmx *os.File

	closeOnce sync.Once
}

// startSession launches argv under a fresh PTY sized to cols x rows. The caller owns the
// read and wait loops.
//
// A non-nil credential runs the shell as that user: their login shell, their home, and
// their uid/gid, so the session behaves like an ssh login rather than like the agent
// wearing someone else's name. Resolving the credential is the caller's job, because
// failing to switch users has to be reported to the browser, not swallowed here.
func startSession(id, shell string, argv []string, cols, rows uint32, dir string, cred *osuser.Credential) (*session, error) {
	if cred != nil {
		if cred.Shell != "" {
			shell = cred.Shell
		}
		if cred.Home != "" {
			dir = cred.Home
		}
	}

	cmd := exec.Command(shell, argv...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.Env = append(cmd.Env, cred.Env()...)
	if dir != "" {
		cmd.Dir = dir
	}
	if attr := cred.SysProcAttr(); attr != nil {
		cmd.SysProcAttr = attr
	}

	ptmx, err := pty.StartWithSize(cmd, winsize(cols, rows))
	if err != nil {
		return nil, err
	}
	return &session{id: id, cmd: cmd, ptmx: ptmx}, nil
}

// write feeds stdin bytes into the terminal.
func (s *session) write(data []byte) {
	if len(data) == 0 {
		return
	}
	_, _ = s.ptmx.Write(data)
}

// resize updates the PTY window size so full-screen programs (vim, htop) reflow.
func (s *session) resize(cols, rows uint32) {
	_ = pty.Setsize(s.ptmx, winsize(cols, rows))
}

// signal forwards a signal to the foreground process. Best-effort.
func (s *session) signal(sig syscall.Signal) {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(sig)
	}
}

// close kills the process and tears the PTY down. Safe to call more than once; the read
// loop unblocks when the PTY is closed.
func (s *session) close() {
	s.closeOnce.Do(func() {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		_ = s.ptmx.Close()
	})
}

func winsize(cols, rows uint32) *pty.Winsize {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	return &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
}

// read returns PTY output; it fails once the process has exited and the PTY closed.
func (s *session) read(buf []byte) (int, error) { return s.ptmx.Read(buf) }

// wait blocks until the process exits and returns its exit code and a message.
func (s *session) wait() (int, string) { return waitResult(s.cmd) }

// shellArgv is the argv for a login-style interactive shell or a one-shot command.
func shellArgv(_ string, command bool, line string) []string {
	if command {
		return []string{"-c", line}
	}
	return []string{"-i"} // interactive shell; the PTY supplies the tty
}

func waitResult(cmd *exec.Cmd) (int, string) {
	err := cmd.Wait()
	if err == nil {
		return 0, ""
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return -int(ws.Signal()), ws.Signal().String()
		}
		return ee.ExitCode(), ""
	}
	return -1, err.Error()
}

func detectShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	for _, c := range []string{"/bin/bash", "/usr/bin/bash", "/bin/sh"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "/bin/sh"
}
