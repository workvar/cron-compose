//go:build !windows

package deploy

import (
	"os"
	"path/filepath"
)

// Swap points current at release atomically: a symlink is written next to the real
// one and renamed over it, so no reader ever sees the link missing.
func (l Layout) Swap(release string) error {
	rel, err := filepath.Rel(l.Base, release)
	if err != nil {
		rel = release // absolute target still works, just less tidy
	}
	tmp := l.Current + ".swapping"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.Symlink(rel, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, l.Current)
}
