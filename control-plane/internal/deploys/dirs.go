package deploys

import (
	"sort"
	"strings"
)

// NormalizeRepoPath trims and normalizes a repo-relative path for API queries.
// Empty stays ""; "." becomes "" (repo root).
func NormalizeRepoPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.Trim(p, "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if p == "." {
		return ""
	}
	return p
}

// DirBaseName returns the last path segment; "" or "." → ".".
func DirBaseName(path string) string {
	path = strings.Trim(path, "/")
	if path == "" || path == "." {
		return "."
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// ShouldSkipDirName reports whether a directory name should be omitted from listings.
func ShouldSkipDirName(name string) bool {
	return name == ".git"
}

// FilterAndCapDirs filters repo-relative directory paths for shallow or recursive listing.
// paths are full repo-relative directory paths already known to be directories.
func FilterAndCapDirs(paths []string, parent string, recursive bool, cap int) (items []DirEntry, truncated bool) {
	parent = NormalizeRepoPath(parent)
	var kept []string
	if recursive {
		prefix := parent
		if prefix != "" {
			prefix += "/"
		}
		for _, p := range paths {
			p = NormalizeRepoPath(p)
			if parent == "" || p == parent || strings.HasPrefix(p, prefix) {
				kept = append(kept, p)
			}
		}
		sort.Strings(kept)
	} else {
		for _, p := range paths {
			p = NormalizeRepoPath(p)
			if parent == "" {
				if !strings.Contains(p, "/") {
					kept = append(kept, p)
				}
			} else {
				want := parent + "/"
				if strings.HasPrefix(p, want) {
					rest := strings.TrimPrefix(p, want)
					if rest != "" && !strings.Contains(rest, "/") {
						kept = append(kept, p)
					}
				}
			}
		}
		sort.Strings(kept)
	}
	for _, p := range kept {
		name := DirBaseName(p)
		if ShouldSkipDirName(name) {
			continue
		}
		items = append(items, DirEntry{Name: name, Path: p})
	}
	if len(items) > cap {
		return items[:cap], true
	}
	return items, false
}
