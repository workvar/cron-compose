//go:build !darwin

package osuser

import "os"

// loginShell reads the user's shell from the passwd database. os/user does not expose
// it, and guessing wrong means a shell session that does not match what the user gets
// over ssh, so an unreadable passwd falls back to /bin/sh rather than to bash.
func loginShell(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return fallbackShell
	}
	for _, line := range splitLines(string(data)) {
		fields := splitColons(line)
		if len(fields) >= 7 && fields[0] == username && fields[6] != "" {
			return fields[6]
		}
	}
	return fallbackShell
}

func splitLines(s string) []string  { return split(s, '\n') }
func splitColons(s string) []string { return split(s, ':') }

func split(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
