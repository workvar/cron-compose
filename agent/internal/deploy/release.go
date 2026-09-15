package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Releases give a project the layout a Capistrano or Vercel deploy has:
//
//	<base>/releases/20260915T131200Z-a1b2c3d/   one immutable checkout per deploy
//	<base>/current -> releases/20260915T131200Z-a1b2c3d
//
// Two things fall out of it. A deploy is atomic: the install runs inside the new
// release directory and nothing serves from it until the symlink moves, so the live
// tree is never half-updated. And a rollback is a symlink swap with no network at
// all, which matters because the commit being rolled back to may have been
// force-pushed away on the remote by the time it is needed.
//
// Process managers are pointed at <base>/current, never at a release directory, so
// their working directory stays valid across deploys.
const (
	releasesDir   = "releases"
	currentLink   = "current"
	shaFile       = ".croncompose-release"
	keepReleases  = 5
	tmpDirPattern = ".incoming-"
)

// Layout resolves the paths for one project base directory.
type Layout struct {
	Base     string // <clone_path>
	Releases string // <clone_path>/releases
	Current  string // <clone_path>/current (a symlink)
}

// NewLayout builds the layout for a base directory. It does not touch the disk.
func NewLayout(base string) Layout {
	base = filepath.Clean(base)
	return Layout{
		Base:     base,
		Releases: filepath.Join(base, releasesDir),
		Current:  filepath.Join(base, currentLink),
	}
}

// releaseName is the directory name for a new release: sortable by time, and
// readable enough that `ls releases/` tells an operator what is deployed.
func releaseName(now time.Time, sha string) string {
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	if short == "" {
		short = "unknown"
	}
	return now.UTC().Format("20060102T150405Z") + "-" + short
}

// NeedsMigration reports whether base is still a plain in-place checkout from before
// releases existed: a git working copy sitting directly at the base path.
func (l Layout) NeedsMigration() bool {
	if _, err := os.Lstat(l.Current); err == nil {
		return false
	}
	return isGitDir(l.Base)
}

// Migrate converts an old in-place checkout into the release layout, keeping the
// existing working copy as the first release so the running process keeps its files.
// The move is done through a sibling path rather than in place, because a directory
// cannot be moved inside itself.
func (l Layout) Migrate(ctx context.Context) (string, error) {
	sha := headCommit(ctx, l.Base)
	staging := l.Base + ".migrating"
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := os.Rename(l.Base, staging); err != nil {
		return "", fmt.Errorf("move existing checkout aside: %w", err)
	}
	if err := os.MkdirAll(l.Releases, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(l.Releases, releaseName(time.Now(), sha))
	if err := os.Rename(staging, dest); err != nil {
		return "", fmt.Errorf("move existing checkout into releases: %w", err)
	}
	if err := writeReleaseSHA(dest, sha); err != nil {
		return "", err
	}
	if err := l.Swap(dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Prepare makes an empty directory for an incoming release. The final name is not
// known until the checkout reveals its commit, so the clone lands in a temporary
// directory that Promote renames.
func (l Layout) Prepare(runID string) (string, error) {
	if err := os.MkdirAll(l.Releases, 0o755); err != nil {
		return "", err
	}
	dir := filepath.Join(l.Releases, tmpDirPattern+sanitizeSegment(runID))
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Promote renames a prepared directory to its final release name and records the
// commit inside it. It returns the final path.
func (l Layout) Promote(tmp, sha string) (string, error) {
	dest := filepath.Join(l.Releases, releaseName(time.Now(), sha))
	for i := 1; ; i++ {
		if _, err := os.Lstat(dest); os.IsNotExist(err) {
			break
		}
		// Same second, same commit: only possible on a rapid redeploy. Suffix rather
		// than overwrite, because the existing directory may be the live release.
		dest = filepath.Join(l.Releases, fmt.Sprintf("%s.%d", releaseName(time.Now(), sha), i))
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	if err := writeReleaseSHA(dest, sha); err != nil {
		return "", err
	}
	return dest, nil
}

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

// CurrentRelease returns the release directory current points at, or "" if there is
// no usable symlink yet.
func (l Layout) CurrentRelease() string {
	target, err := os.Readlink(l.Current)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(l.Base, target)
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		return ""
	}
	return filepath.Clean(target)
}

// FindRelease returns the newest release directory holding sha, or "" when the
// commit is not on disk. This is what makes a rollback free of the network.
func (l Layout) FindRelease(sha string) string {
	if sha == "" {
		return ""
	}
	entries, err := os.ReadDir(l.Releases)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), tmpDirPattern) {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names))) // newest first, names sort by time
	for _, name := range names {
		dir := filepath.Join(l.Releases, name)
		if readReleaseSHA(dir) == sha {
			return dir
		}
	}
	return ""
}

// Prune deletes all but the newest keepReleases directories, never touching the live
// one. Errors are returned for logging but are not fatal to a deploy.
func (l Layout) Prune() []string {
	entries, err := os.ReadDir(l.Releases)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), tmpDirPattern) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keepReleases {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	live := l.CurrentRelease()
	var removed []string
	for _, name := range names[keepReleases:] {
		dir := filepath.Join(l.Releases, name)
		if dir == live {
			continue
		}
		if err := os.RemoveAll(dir); err == nil {
			removed = append(removed, name)
		}
	}
	return removed
}

func writeReleaseSHA(dir, sha string) error {
	if sha == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, shaFile), []byte(sha+"\n"), 0o644)
}

func readReleaseSHA(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, shaFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// sanitizeSegment keeps a caller-supplied id from escaping the releases directory.
func sanitizeSegment(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s)
	if s == "" {
		return "run"
	}
	return s
}
