//go:build !windows

package deploy

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// chownTree hands path and everything under it to cred. Entries that already
// belong to the user are skipped: `chown -R` rewrote every inode (a metadata
// write per file even when nothing changed), which on an SD card made a
// node_modules tree take longer to chown than to restore. Symlinks are changed
// themselves, never followed.
func chownTree(path string, cred *osuser.Credential) error {
	if cred == nil {
		return nil
	}
	uid, gid := int(cred.UID), int(cred.GID)
	var firstErr error
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if err := lchownIfNeeded(p, uid, gid); err != nil && firstErr == nil {
			firstErr = err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return firstErr
}

// chownShallow hands one entry (not its children) to cred. A missing path is
// not an error.
func chownShallow(path string, cred *osuser.Credential) error {
	if cred == nil {
		return nil
	}
	err := lchownIfNeeded(path, int(cred.UID), int(cred.GID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func lchownIfNeeded(path string, uid, gid int) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == uid && int(st.Gid) == gid {
		return nil
	}
	return os.Lchown(path, uid, gid)
}
