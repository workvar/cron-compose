//go:build !windows

package platform

// HasBash reports whether a bash is available. Unix installs always ship one.
func HasBash() bool { return true }
