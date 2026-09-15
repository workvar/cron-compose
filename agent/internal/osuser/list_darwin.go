package osuser

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// listAccounts asks Directory Services for every account, the same reason
// shell_darwin.go's loginShell does not read /etc/passwd: that file holds only stub
// system accounts on macOS, real accounts live in Open Directory.
//
// `dscl . -readall /Users uid,home,shell` prints one paragraph per user; blank lines
// separate records. The lookup is bounded because it can talk to a directory server
// on a domain-joined Mac, and the agent must not hang listing users.
func listAccounts() ([]account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "dscl", "-plist", ".", "-readall", "/Users",
		"UniqueID", "NFSHomeDirectory", "UserShell").Output()
	if err != nil {
		return nil, err
	}
	return parseDSClPlist(out), nil
}

// parseDSClPlist pulls RecordName/UniqueID/NFSHomeDirectory/UserShell out of the
// plist -readall dumps. It is a small hand-rolled scan rather than a full plist
// decoder: the output shape here is stable (flat dict-of-arrays per record) and the
// agent has no other reason to carry a plist dependency.
func parseDSClPlist(plist []byte) []account {
	type rec struct{ name, uid, home, shell string }
	var recs []rec
	cur := rec{}
	inArray := false
	var arrayKey string

	lines := strings.Split(string(plist), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "<dict>":
			cur = rec{}
		case line == "</dict>":
			if cur.name != "" {
				recs = append(recs, cur)
			}
		case strings.HasPrefix(line, "<key>"):
			arrayKey = strings.TrimSuffix(strings.TrimPrefix(line, "<key>"), "</key>")
			inArray = true
		case line == "<array>":
			// no-op, value(s) follow
		case line == "</array>":
			inArray = false
		case strings.HasPrefix(line, "<string>") && inArray:
			val := strings.TrimSuffix(strings.TrimPrefix(line, "<string>"), "</string>")
			switch arrayKey {
			case "RecordName":
				if cur.name == "" {
					cur.name = val
				}
			case "UniqueID":
				cur.uid = val
			case "NFSHomeDirectory":
				cur.home = val
			case "UserShell":
				cur.shell = val
			}
		}
	}

	accounts := make([]account, 0, len(recs))
	for _, r := range recs {
		if r.name == "" || r.uid == "" {
			continue
		}
		uid, err := strconv.ParseUint(r.uid, 10, 32)
		if err != nil {
			continue
		}
		accounts = append(accounts, account{
			Username: r.name,
			UID:      uint32(uid),
			Home:     r.home,
			Shell:    r.shell,
		})
	}
	return accounts
}
