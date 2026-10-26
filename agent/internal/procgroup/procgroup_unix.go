//go:build !windows

package procgroup

import "os"

// Group is a no-op on Unix; see the package comment.
type Group struct{}

// Attach returns a no-op group. Limits are enforced by systemd-run on Unix.
func Attach(_ *os.Process, _ Limits) (*Group, error) { return &Group{}, nil }

// Supported reports whether Attach enforces limits on this platform.
func Supported() bool { return false }

// Kill is a no-op on Unix.
func (g *Group) Kill() error { return nil }

// Close is a no-op on Unix.
func (g *Group) Close() error { return nil }
