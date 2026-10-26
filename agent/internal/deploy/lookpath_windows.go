//go:build windows

package deploy

import (
	"fmt"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	"github.com/croncompose/croncompose/agent/internal/platform"
)

// lookPathAs finds bin for a Windows deploy. There is no other user's login shell
// to consult (the agent never switches users on Windows), so this is the agent's
// PATH plus the per-user install locations a service PATH misses.
func lookPathAs(_ *osuser.Credential, bin string) (string, error) {
	if bin == "" || strings.ContainsAny(bin, " \t\n`$\\\"'") {
		return "", fmt.Errorf("invalid binary name")
	}
	if p, err := platform.LookPath(bin); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found", bin)
}
