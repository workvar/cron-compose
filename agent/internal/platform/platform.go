// Package platform hides the handful of facts the agent needs about the host OS:
// which bash to run scripts with, whether the process is privileged, where scratch
// space lives, and how to turn a control-plane path into a local one.
//
// Everything else in the agent stays OS-neutral by asking this package instead of
// hardcoding /bin/bash, os.Geteuid() == 0, or "/tmp". Linux and macOS behave exactly
// as before; Windows gets Git Bash, an elevation check, and drive-rooted paths.
package platform

import "runtime"

// IsWindows reports whether the agent was built for Windows.
func IsWindows() bool { return runtime.GOOS == "windows" }

// BashHint is appended to errors when no usable bash can be found, so the operator
// knows what to install rather than seeing a bare "executable file not found".
const BashHint = "install Git for Windows (https://git-scm.com/download/win), which ships bash, " +
	"or set AGENT_BASH to the full path of bash.exe"
