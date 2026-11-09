//go:build windows

package deploy

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// chownTree keeps the previous behavior on Windows, where a deploy normally runs
// without a user switch (cred is nil) and this is a no-op.
func chownTree(path string, cred *osuser.Credential) error {
	if cred == nil {
		return nil
	}
	cmd := exec.Command("chown", "-R", fmt.Sprintf("%d:%d", cred.UID, cred.GID), path)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("chown %s: %s", path, msg)
	}
	return nil
}

// chownShallow matches chownTree on Windows.
func chownShallow(path string, cred *osuser.Credential) error {
	return chownTree(path, cred)
}
