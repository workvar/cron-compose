//go:build windows

package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Swap points current at release. Windows cannot rename one directory link over
// another, so the old link is removed first and the new one created in its place;
// the gap is a few microseconds. A symbolic link needs SeCreateSymbolicLinkPrivilege
// (held by the LocalSystem service account); without it we fall back to an NTFS
// junction, which any user may create. Removing a link never touches its target.
func (l Layout) Swap(release string) error {
	rel, err := filepath.Rel(l.Base, release)
	if err != nil {
		rel = release
	}
	if fi, err := os.Lstat(l.Current); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 && fi.Mode()&os.ModeIrregular == 0 && fi.IsDir() {
			return fmt.Errorf("%s is a real directory, not a release link; refusing to replace it", l.Current)
		}
		if err := os.Remove(l.Current); err != nil {
			return fmt.Errorf("remove old current link: %w", err)
		}
	}
	if err := os.Symlink(rel, l.Current); err == nil {
		return nil
	}
	abs, err := filepath.Abs(release)
	if err != nil {
		return err
	}
	out, err := exec.Command("cmd.exe", "/D", "/C", "mklink", "/J", l.Current, abs).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create junction %s: %v: %s", l.Current, err, out)
	}
	return nil
}
