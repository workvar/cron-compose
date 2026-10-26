//go:build windows

package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	"github.com/croncompose/croncompose/agent/internal/procgroup"
)

// session is one process attached to a Windows pseudo console (ConPTY, Windows 10
// 1809 / Server 2019 and later). ConPTY speaks VT sequences, so the browser's
// xterm.js renders PowerShell exactly like it renders bash on Linux.
type session struct {
	id    string
	hpc   windows.Handle
	in    *os.File // we write keystrokes here
	out   *os.File // we read rendered output here
	proc  windows.Handle
	group *procgroup.Group

	done     chan struct{}
	exitCode uint32

	closeOnce sync.Once
	hpcOnce   sync.Once
}

// startSession launches shell+argv under a new pseudo console. cred is always nil
// on Windows (osuser.Resolve refuses other users before we get here).
func startSession(id, shell string, argv []string, cols, rows uint32, dir string, _ *osuser.Credential) (*session, error) {
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		closeHandles(inR, inW)
		return nil, err
	}
	var hpc windows.Handle
	err := windows.CreatePseudoConsole(coord(cols, rows), inR, outW, 0, &hpc)
	// The pseudo console holds its own copies of the far ends of both pipes.
	closeHandles(inR, outW)
	if err != nil {
		closeHandles(inW, outR)
		return nil, err
	}
	s := &session{
		id:   id,
		hpc:  hpc,
		in:   os.NewFile(uintptr(inW), "conpty-in"),
		out:  os.NewFile(uintptr(outR), "conpty-out"),
		done: make(chan struct{}),
	}
	if err := s.spawn(shell, argv, dir); err != nil {
		s.closeConsole()
		_ = s.in.Close()
		_ = s.out.Close()
		return nil, err
	}
	go s.reap()
	return s, nil
}

func (s *session) spawn(shell string, argv []string, dir string) error {
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return err
	}
	defer attrs.Delete()
	// The attribute value is the HPCON itself, not a pointer to it.
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		*(*unsafe.Pointer)(unsafe.Pointer(&s.hpc)), unsafe.Sizeof(s.hpc)); err != nil {
		return err
	}
	si := &windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(*si))
	// Null std handles force the child onto the pseudo console even though the
	// agent itself may have redirected stdio (it does under the service manager).
	si.Flags = windows.STARTF_USESTDHANDLES

	cmdLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{shell}, argv...)))
	if err != nil {
		return err
	}
	var dirPtr *uint16
	if dir != "" {
		if dirPtr, err = windows.UTF16PtrFromString(dir); err != nil {
			return err
		}
	}
	env := envBlock(append(os.Environ(), "TERM=xterm-256color"))
	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(nil, cmdLine, nil, nil, false, flags,
		&env[0], dirPtr, &si.StartupInfo, &pi); err != nil {
		return err
	}
	_ = windows.CloseHandle(pi.Thread)
	s.proc = pi.Process
	if p, err := os.FindProcess(int(pi.ProcessId)); err == nil {
		s.group, _ = procgroup.Attach(p, procgroup.Limits{})
	}
	return nil
}

// reap waits for the shell to exit, then closes the pseudo console so the pending
// read on the output pipe returns EOF (ConPTY does not close it on its own).
func (s *session) reap() {
	_, _ = windows.WaitForSingleObject(s.proc, windows.INFINITE)
	_ = windows.GetExitCodeProcess(s.proc, &s.exitCode)
	s.closeConsole()
	close(s.done)
}

func (s *session) read(buf []byte) (int, error) { return s.out.Read(buf) }

func (s *session) wait() (int, string) {
	<-s.done
	return int(s.exitCode), ""
}

func (s *session) write(data []byte) {
	if len(data) > 0 {
		_, _ = s.in.Write(data)
	}
}

func (s *session) resize(cols, rows uint32) {
	_ = windows.ResizePseudoConsole(s.hpc, coord(cols, rows))
}

// signal maps Unix signal names onto what a console understands: INT and QUIT are
// Ctrl-C and Ctrl-Break keystrokes; anything stronger ends the process tree.
func (s *session) signal(sig syscall.Signal) {
	switch sig {
	case syscall.SIGINT:
		s.write([]byte{0x03})
	case syscall.SIGQUIT:
		s.write([]byte{0x1c})
	default:
		s.kill()
	}
}

func (s *session) close() {
	s.closeOnce.Do(func() {
		s.kill()
		s.closeConsole()
		_ = s.in.Close()
		_ = s.out.Close()
	})
}

func (s *session) kill() {
	if s.group != nil {
		_ = s.group.Kill()
	}
	if s.proc != 0 {
		_ = windows.TerminateProcess(s.proc, 1)
	}
}

func (s *session) closeConsole() {
	s.hpcOnce.Do(func() { windows.ClosePseudoConsole(s.hpc) })
}

// shellArgv builds the argv for an interactive shell or a one-shot command,
// according to which shell it is.
func shellArgv(shell string, command bool, line string) []string {
	switch strings.ToLower(strings.TrimSuffix(filepath.Base(shell), filepath.Ext(shell))) {
	case "cmd":
		if command {
			return []string{"/C", line}
		}
		return nil
	case "bash", "sh":
		if command {
			return []string{"-lc", line}
		}
		return []string{"-li"}
	default: // powershell, pwsh
		if command {
			return []string{"-NoLogo", "-NoProfile", "-Command", line}
		}
		return []string{"-NoLogo"}
	}
}

func detectShell() string { return osuser.DefaultShell() }

func coord(cols, rows uint32) windows.Coord {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	return windows.Coord{X: int16(cols), Y: int16(rows)}
}

// envBlock encodes env as the double-NUL-terminated UTF-16 block CreateProcess wants.
func envBlock(env []string) []uint16 {
	var b []uint16
	for _, kv := range env {
		if strings.IndexByte(kv, 0) >= 0 {
			continue
		}
		b = append(b, windows.StringToUTF16(kv)...) // includes its own NUL
	}
	return append(b, 0)
}

func closeHandles(hs ...windows.Handle) {
	for _, h := range hs {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
	}
}
