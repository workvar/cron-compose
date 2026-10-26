//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the agent runs with an elevated token. The Windows
// service runs as LocalSystem, which is the Windows equivalent of root.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// TempDir is the shared scratch directory (C:\Windows\Temp for LocalSystem).
func TempDir() string { return os.TempDir() }

// LocalPath maps a control-plane path to this host. The control plane speaks Unix
// paths ("/opt/apps/web", "/tmp"); on Windows a rooted path with no drive letter is
// placed on the system drive, so "/opt/apps/web" becomes C:\opt\apps\web no matter
// what the service's working directory is. Paths that already carry a drive letter
// or UNC prefix are only cleaned.
func LocalPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if p == "/tmp" || strings.HasPrefix(p, "/tmp/") {
		return filepath.Join(TempDir(), filepath.FromSlash(strings.TrimPrefix(p, "/tmp")))
	}
	if filepath.VolumeName(p) != "" {
		return filepath.Clean(p)
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return filepath.Join(systemDrive()+`\`, filepath.FromSlash(p))
	}
	return filepath.Clean(filepath.FromSlash(p))
}

func systemDrive() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return d
	}
	return "C:"
}

// FreeBytes reports the bytes available to the caller on dir's volume.
func FreeBytes(dir string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var avail, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &free); err != nil {
		return 0, err
	}
	return avail, nil
}

// ExtraEnv keeps Git Bash in the caller's directory (its /etc/profile otherwise
// cds to $HOME for login shells) and lets it inherit the full Windows PATH.
func ExtraEnv() []string {
	return []string{"CHERE_INVOKING=1", "MSYS2_PATH_TYPE=inherit"}
}

// PassthroughEnv lists agent environment keys a sandboxed job still inherits.
// Windows programs break in surprising ways without SystemRoot, PATHEXT, and the
// profile directories, so these survive even the executor's minimal environment.
func PassthroughEnv() []string {
	return []string{
		"HOME", "LANG", "TZ",
		"SystemRoot", "SystemDrive", "windir", "ComSpec", "PATHEXT",
		"TEMP", "TMP", "USERPROFILE", "USERNAME", "USERDOMAIN", "COMPUTERNAME",
		"APPDATA", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)",
		"ProgramW6432", "CommonProgramFiles", "NUMBER_OF_PROCESSORS",
		"PROCESSOR_ARCHITECTURE", "OS", "PSModulePath", "PM2_HOME",
	}
}

// CurrentUser is the agent's account name as the environment reports it
// (SYSTEM's USERNAME is the machine account, e.g. "HOST$").
func CurrentUser() string { return os.Getenv("USERNAME") }
