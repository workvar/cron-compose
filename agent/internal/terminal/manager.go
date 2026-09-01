// Package terminal runs interactive shells and one-shot commands on behalf of the
// control plane's web terminal, streaming output back as TerminalOutput messages.
//
// It is Unix-only: it relies on a pseudo-terminal (creack/pty), consistent with the
// rest of the agent. Output is sent through a Sender that the runtime wires to its
// EPHEMERAL direct-send path (never the durable outbox), so terminal bytes are real-time
// and never persisted or replayed on reconnect.
package terminal

import (
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Sender ships one TerminalOutput back to the control plane.
type Sender func(*agentv1.TerminalOutput)

// Manager owns all live terminal sessions for this agent, keyed by session id.
type Manager struct {
	log   *slog.Logger
	send  Sender
	shell string
	home  string

	mu   sync.Mutex
	sess map[string]*session
}

// NewManager builds a Manager. The login shell is taken from $SHELL when set.
func NewManager(log *slog.Logger, send Sender) *Manager {
	return &Manager{
		log:   log,
		send:  send,
		shell: detectShell(),
		home:  os.Getenv("HOME"),
		sess:  map[string]*session{},
	}
}

// Handle routes one inbound TerminalInput.
func (m *Manager) Handle(in *agentv1.TerminalInput) {
	switch in.GetOp() {
	case "open":
		m.open(in)
	case "data":
		m.with(in.GetSessionId(), func(s *session) { s.write(in.GetData()) })
	case "resize":
		m.with(in.GetSessionId(), func(s *session) { s.resize(in.GetCols(), in.GetRows()) })
	case "signal":
		m.with(in.GetSessionId(), func(s *session) { s.signal(signalFor(in.GetSignal())) })
	case "close":
		m.closeSession(in.GetSessionId())
	}
}

// CloseAll tears down every session. Called when the control-plane stream drops so no
// orphan shells linger.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	all := make([]*session, 0, len(m.sess))
	for _, s := range m.sess {
		all = append(all, s)
	}
	m.sess = map[string]*session{}
	m.mu.Unlock()
	for _, s := range all {
		s.close()
	}
}

func (m *Manager) open(in *agentv1.TerminalInput) {
	id := in.GetSessionId()
	var argv []string
	if in.GetKind() == "command" {
		argv = []string{"-c", in.GetCommand()}
	} else {
		argv = []string{"-i"} // interactive shell; the PTY supplies the tty
	}

	// Resolve the requested user before starting anything: a session that cannot run
	// as who it claims to must fail loudly rather than fall back to the agent's user.
	cred, err := osuser.Resolve(in.GetRunAs())
	if err != nil {
		m.emit(id, "error", nil, 0, "run as "+in.GetRunAs()+": "+err.Error())
		return
	}

	sess, err := startSession(id, m.shell, argv, in.GetCols(), in.GetRows(), m.home, cred)
	if err != nil {
		m.emit(id, "error", nil, 0, err.Error())
		return
	}

	m.mu.Lock()
	m.sess[id] = sess
	m.mu.Unlock()

	m.emit(id, "started", nil, 0, "")
	go m.pump(sess)
}

// pump streams PTY output until the process exits, then reports the exit status.
func (m *Manager) pump(s *session) {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			m.emit(s.id, "data", chunk, 0, "")
		}
		if err != nil {
			break
		}
	}
	code, msg := waitResult(s.cmd)
	m.remove(s.id)
	m.emit(s.id, "exit", nil, int32(code), msg)
}

func (m *Manager) with(id string, fn func(*session)) {
	m.mu.Lock()
	s := m.sess[id]
	m.mu.Unlock()
	if s != nil {
		fn(s)
	}
}

func (m *Manager) closeSession(id string) {
	m.mu.Lock()
	s := m.sess[id]
	delete(m.sess, id)
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (m *Manager) remove(id string) {
	m.mu.Lock()
	delete(m.sess, id)
	m.mu.Unlock()
}

func (m *Manager) emit(id, kind string, data []byte, code int32, msg string) {
	m.send(&agentv1.TerminalOutput{
		SessionId: id,
		Kind:      kind,
		Data:      data,
		ExitCode:  code,
		Message:   msg,
	})
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

func signalFor(name string) syscall.Signal {
	switch name {
	case "TERM":
		return syscall.SIGTERM
	case "QUIT":
		return syscall.SIGQUIT
	case "KILL":
		return syscall.SIGKILL
	case "HUP":
		return syscall.SIGHUP
	default:
		return syscall.SIGINT
	}
}
