package deploy

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// copyAppRoot copies the contents of src into dst, skipping VCS metadata that
// must never land in a release (.git). dst is created if missing.
func copyAppRoot(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	st, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("app root %s: %w", src, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("app root %s is not a directory", src)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Never ship the git database into a release.
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// normalizeRoot turns ".", "", "./web", "/web" into a clean relative path.
func normalizeRoot(root string) string {
	root = strings.TrimSpace(root)
	root = strings.ReplaceAll(root, "\\", "/")
	root = strings.TrimPrefix(root, "/")
	root = strings.TrimPrefix(root, "./")
	root = filepath.Clean(root)
	if root == "." || root == "" {
		return "."
	}
	if strings.HasPrefix(root, "..") {
		return "."
	}
	return root
}
