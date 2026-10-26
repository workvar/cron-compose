//go:build !windows

package platform

import "os/exec"

// LookPath is exec.LookPath on Unix; login-shell lookups (nvm and friends) are
// handled by the callers that need them.
func LookPath(name string) (string, error) { return exec.LookPath(name) }

// ExtraBinDirs has nothing to add on Unix.
func ExtraBinDirs() []string { return nil }
