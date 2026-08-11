package terminal

import (
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
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
func startSession(id, shell string, argv []string, cols, rows uint32, dir string) (*session, error) {
	cmd := exec.Command(shell, argv...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if dir != "" {
		cmd.Dir = dir
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
