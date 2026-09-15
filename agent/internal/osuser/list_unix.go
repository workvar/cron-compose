//go:build !darwin

package osuser

import (
	"os"
	"strconv"
)

// listAccounts reads every account straight out of /etc/passwd. Unlike macOS,
// Linux's passwd file is the actual account database, so no directory service round
// trip is needed here (contrast shell_unix.go's loginShell, which is the same file
// for the same reason).
func listAccounts() ([]account, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}

	var out []account
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		fields := splitColons(line)
		if len(fields) < 7 {
			continue
		}
		uid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue // malformed row; skip rather than fail the whole list
		}
		out = append(out, account{
			Username: fields[0],
			UID:      uint32(uid),
			Home:     fields[5],
			Shell:    fields[6],
		})
	}
	return out, nil
}
