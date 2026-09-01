package connectors

import "os"

// systemdAvailable reports whether this host is actually running systemd, which is a
// stricter question than whether systemctl is on PATH.
//
// Two hosts fail this in different ways. A container routinely ships the systemd
// package without running it as PID 1, so systemctl exists and every call fails with
// "System has not been booted with systemd as init system". macOS has no systemctl at
// all and uses launchd. In both cases the connector must say the operation is
// unsupported here rather than attempting it and reporting the resulting exec error as
// though the service itself had failed.
//
// /run/systemd/system is the same marker systemctl itself uses.
func systemdAvailable() bool {
	if !has("systemctl") {
		return false
	}
	st, err := os.Stat("/run/systemd/system")
	return err == nil && st.IsDir()
}

// noSystemd is the standard refusal for an action that has no meaning without a
// service manager we can drive. It names the alternative so the operator is not left
// guessing why a Mac or a container reports less than a Debian box.
func noSystemd(what string) Result {
	return fail(StatusUnsupported,
		"cannot "+what+" on this host: it is not running systemd "+
			"(macOS uses launchd, and containers often run no service manager at all)")
}
