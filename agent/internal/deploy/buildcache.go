package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

// Deploy installs and builds run inside a fresh release directory, so a Next.js
// install or a Go compile would otherwise repeat on every deploy. The cache
// lives at <base>/.cc-cache, next to releases/, and prune does not touch it.
//
// An install step is keyed by the dependency manifests and the install command.
// A source-only change reuses node_modules (or vendor) and still compiles.
// A build step is keyed by the source tree, the build command, and the app
// env, so an unchanged app restores its binary or .next output instead of
// compiling. npm is asked to prefer its existing cache; Go, pnpm, and Cargo
// keep the deploy user's own caches, which are already warm.
const (
	cacheDirName     = ".cc-cache"
	cacheMarker      = "complete"
	keepCacheEntries = 2
)

// stepCachePlan is the fingerprint taken before the script runs. Saving must
// reuse it: hashing again after the script would include the outputs and the
// next clean release would never match.
type stepCachePlan struct {
	base     string
	appKey   string
	kind     string // install | build
	fp       string
	script   string
	language string
}

// splitCacheable reports whether script is a pure "install && build" chain.
// Mixed commands stay intact so a cache hit cannot skip a copy or a migration.
func splitCacheable(script, language string) (install, build string, ok bool) {
	inst, bld := splitInstallBuild(script, language)
	if inst == "" || bld == "" {
		return "", "", false
	}
	if commandsKind(inst) != "install" || commandsKind(bld) != "build" {
		return "", "", false
	}
	return inst, bld, true
}

// commandsKind is "install" or "build" when every command in script is that
// kind. A single unrecognized command makes the script uncacheable.
func commandsKind(script string) string {
	parts := splitShellCommands(script)
	if len(parts) == 0 {
		return ""
	}
	kind := ""
	for _, p := range parts {
		k := classifyInstallKind(p, "")
		if k != "install" && k != "build" {
			return ""
		}
		if kind == "" {
			kind = k
			continue
		}
		if kind != k {
			return ""
		}
	}
	return kind
}

func planStepCache(j appScript) (stepCachePlan, bool) {
	if j.flowPhase != "install" && j.flowPhase != "build" {
		return stepCachePlan{}, false
	}
	kind := commandsKind(j.script)
	if kind == "" {
		return stepCachePlan{}, false
	}
	if j.flowPhase == "build" && kind != "build" {
		return stepCachePlan{}, false
	}
	if kind == "install" && !hasManifest(j.work) {
		return stepCachePlan{}, false
	}
	base := projectBaseFromWork(j.work)
	if base == "" {
		return stepCachePlan{}, false
	}
	fp, err := stepFingerprint(j.work, kind, j.script, j.language, j.env)
	if err != nil || fp == "" {
		return stepCachePlan{}, false
	}
	return stepCachePlan{
		base:     base,
		appKey:   cacheAppKey(j.app, j.work),
		kind:     kind,
		fp:       fp,
		script:   j.script,
		language: j.language,
	}, true
}

func (m *Manager) replayCachedStep(runID, token, logPhase, work string, plan stepCachePlan, cred *osuser.Credential) bool {
	entry := cacheEntryPath(plan)
	if !cacheComplete(entry) {
		return false
	}
	if err := restoreStepCache(entry, work); err != nil {
		if m.log != nil {
			m.log.Warn("deploy cache restore failed", "err", err, "kind", plan.kind, "app", plan.appKey)
		}
		return false
	}
	chownRestored(entry, work, cred)
	m.phaseLine(runID, token, logPhase, "cache hit (in "+work+")")
	return true
}

// warmCachedStep copies the previous build's compiler cache into this release
// before a cache miss, so next build and similar tools recompile less. It does
// not restore node_modules: those are hardlinked on a hit, and warming them
// before a fresh install would let the installer rewrite the previous entry.
func (m *Manager) warmCachedStep(runID, token, work string, plan stepCachePlan) {
	if plan.kind != "build" {
		return
	}
	entry := latestCacheEntry(plan.base, plan.appKey, "build")
	if entry == "" {
		return
	}
	if warmRestore(entry, work, []string{filepath.Join(".next", "cache"), ".turbo"}) == 0 {
		return
	}
	m.logLine(runID, token, "[in "+work+"] reusing compiler cache")
}

func (m *Manager) storeCachedStep(work, language, script string, plan stepCachePlan) {
	if err := saveStepCache(plan, work, language, script); err != nil && m.log != nil {
		m.log.Warn("deploy cache save failed", "err", err, "kind", plan.kind, "app", plan.appKey)
	}
}

func projectBaseFromWork(work string) string {
	dir := filepath.Clean(work)
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		// dir is the release id when its parent is the releases directory.
		// An app folder named "releases" is not the layout directory.
		if filepath.Base(parent) == releasesDir {
			return filepath.Dir(parent)
		}
		dir = parent
	}
}

func cacheAppKey(name, work string) string {
	name = sanitizeKey(name)
	if name == "" {
		name = "app"
	}
	rel := releaseRelative(work)
	rel = sanitizeKey(rel)
	if rel != "" && rel != name {
		name = name + "-" + rel
	}
	return name
}

func releaseRelative(work string) string {
	dir := filepath.Clean(work)
	release := ""
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		if filepath.Base(parent) == releasesDir {
			release = dir
			break
		}
		dir = parent
	}
	rel, err := filepath.Rel(release, work)
	if err != nil || rel == "." {
		return ""
	}
	return rel
}

func sanitizeKey(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" || s == "." || s == ".." {
		return ""
	}
	return s
}

func cacheEntryPath(plan stepCachePlan) string {
	return filepath.Join(plan.base, cacheDirName, "apps", plan.appKey, plan.kind, plan.fp)
}

func cacheComplete(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, cacheMarker))
	return err == nil && !st.IsDir()
}

var manifestNames = []string{
	"package.json", "package-lock.json", "npm-shrinkwrap.json",
	"pnpm-lock.yaml", "pnpm-workspace.yaml", "yarn.lock",
	"bun.lock", "bun.lockb",
	"go.mod", "go.sum", "go.work", "go.work.sum",
	"Cargo.toml", "Cargo.lock",
	"requirements.txt", "poetry.lock", "Pipfile.lock", "pyproject.toml", "uv.lock",
	"Gemfile", "Gemfile.lock",
	"composer.json", "composer.lock",
	"packages.lock.json", "nuget.config", "global.json",
}

var installEnvKeys = []string{"NODE_ENV", "NPM_CONFIG_PRODUCTION", "npm_config_production"}

func hasManifest(work string) bool {
	for _, name := range manifestNames {
		st, err := os.Stat(filepath.Join(work, name))
		if err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func stepFingerprint(work, kind, script, language string, env map[string]string) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "v1\nkind=%s\nlanguage=%s\nscript=%s\n", kind, strings.ToLower(strings.TrimSpace(language)), script)
	switch kind {
	case "install":
		if err := hashNamed(h, work, manifestNames); err != nil {
			return "", err
		}
		hashEnv(h, env, installEnvKeys)
	default:
		if err := hashSourceTree(h, work); err != nil {
			return "", err
		}
		hashEnv(h, env, nil)
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

func hashNamed(h io.Writer, work string, names []string) error {
	for _, name := range names {
		fmt.Fprintf(h, "file %s\n", name)
		b, err := os.ReadFile(filepath.Join(work, name))
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(h, "missing\n")
				continue
			}
			return err
		}
		if _, err := h.Write(b); err != nil {
			return err
		}
		fmt.Fprint(h, "\n")
	}
	return nil
}

func hashEnv(h io.Writer, env map[string]string, only []string) {
	keys := make([]string, 0)
	if len(only) == 0 {
		for k := range env {
			keys = append(keys, k)
		}
	} else {
		keys = append(keys, only...)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := env[k]
		if !ok {
			continue
		}
		fmt.Fprintf(h, "env %s=%s\n", k, v)
	}
}

// skipSourceDirs are produced by install/build or are not inputs. They must be
// ignored so the fingerprint taken before the script matches the one that would
// be taken after it, and so node_modules does not dominate the hash.
var skipSourceDirs = map[string]bool{
	".git": true, ".github": true, "node_modules": true, "vendor": true,
	".venv": true, "venv": true, "__pycache__": true, ".pytest_cache": true,
	".next": true, ".nuxt": true, ".output": true, ".svelte-kit": true,
	"dist": true, "build": true, "out": true, "target": true, "bin": true,
	"coverage": true, ".turbo": true, ".cache": true, cacheDirName: true,
}

func hashSourceTree(h io.Writer, work string) error {
	var files []string
	err := filepath.WalkDir(work, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(work, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if skipSourceDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, rel := range files {
		if err := hashOne(h, work, rel); err != nil {
			return err
		}
	}
	return nil
}

func hashOne(h io.Writer, work, rel string) error {
	path := filepath.Join(work, rel)
	fmt.Fprintf(h, "file %s\n", filepath.ToSlash(rel))
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "link %s\n", target)
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	fmt.Fprint(h, "\n")
	return nil
}

func depRelNames(language string) []string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return []string{"vendor"}
	case "rust":
		return nil
	case "python", "fastapi", "django", "flask":
		return []string{".venv", "venv"}
	case "ruby", "rails":
		return []string{"vendor"}
	case "php", "laravel":
		return []string{"vendor"}
	case "dotnet", "csharp", "aspnet", "aspnetcore":
		return nil
	default:
		return []string{"node_modules"}
	}
}

func buildRelNames(language string) []string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "nextjs":
		return []string{".next"}
	case "nuxt":
		return []string{".nuxt", ".output"}
	case "sveltekit":
		return []string{".svelte-kit", "build"}
	case "astro":
		return []string{"dist"}
	case "react", "vue", "remix":
		return []string{"dist", "build"}
	case "nestjs", "typescript", "javascript", "node", "express", "bun":
		return []string{"dist", "build"}
	case "rust":
		return []string{filepath.Join("target", "release")}
	case "go", "golang":
		return []string{"bin"}
	case "dotnet", "csharp", "aspnet", "aspnetcore":
		return []string{"out", "publish"}
	case "java", "kotlin", "spring":
		return []string{"target"}
	default:
		return []string{"dist", "build", ".next", "out"}
	}
}

func artifactRels(work, kind, language, script string) []string {
	var rels []string
	if kind == "install" {
		rels = append(rels, depRelNames(language)...)
	}
	if kind == "build" {
		rels = append(rels, buildRelNames(language)...)
		rels = append(rels, scriptOutputRels(script)...)
		if b := implicitGoBinary(script, work); b != "" {
			rels = append(rels, b)
		}
	}
	return existingRels(work, rels)
}

func existingRels(work string, rels []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, rel := range rels {
		rel = filepath.Clean(rel)
		if rel == "" || rel == "." || seen[rel] || strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(work, rel)); err != nil {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
	}
	return out
}

func scriptOutputRels(script string) []string {
	var out []string
	for _, part := range splitShellCommands(script) {
		fields := strings.Fields(part)
		for i := 0; i < len(fields); i++ {
			f := strings.Trim(fields[i], `"'`)
			switch {
			case f == "-o" || f == "--output" || f == "-out":
				if i+1 < len(fields) {
					if rel := cleanRelOutput(fields[i+1]); rel != "" {
						out = append(out, rel)
					}
				}
			case strings.HasPrefix(f, "-o="):
				if rel := cleanRelOutput(strings.TrimPrefix(f, "-o=")); rel != "" {
					out = append(out, rel)
				}
			case strings.HasPrefix(f, "--output="):
				if rel := cleanRelOutput(strings.TrimPrefix(f, "--output=")); rel != "" {
					out = append(out, rel)
				}
			}
		}
	}
	return out
}

func cleanRelOutput(p string) string {
	p = strings.Trim(p, `"'`)
	if p == "" || filepath.IsAbs(p) {
		return ""
	}
	p = filepath.Clean(p)
	if p == "." || strings.HasPrefix(p, "..") {
		return ""
	}
	return p
}

func implicitGoBinary(script, work string) string {
	if !strings.Contains(strings.ToLower(script), "go build") {
		return ""
	}
	if len(scriptOutputRels(script)) > 0 {
		return ""
	}
	name := filepath.Base(work)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	if _, err := os.Lstat(filepath.Join(work, name)); err == nil {
		return name
	}
	if runtime.GOOS == "windows" {
		exe := name + ".exe"
		if _, err := os.Lstat(filepath.Join(work, exe)); err == nil {
			return exe
		}
	}
	return ""
}

func saveStepCache(plan stepCachePlan, work, language, script string) error {
	if plan.fp == "" || plan.base == "" {
		return nil
	}
	rels := artifactRels(work, plan.kind, language, script)
	if len(rels) == 0 {
		return nil
	}
	parent := filepath.Dir(cacheEntryPath(plan))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	final := cacheEntryPath(plan)
	tmp := final + ".partial"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	for _, rel := range rels {
		src := filepath.Join(work, rel)
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
		st, err := os.Lstat(src)
		if err != nil {
			continue
		}
		var copyErr error
		if st.IsDir() {
			copyErr = cloneTree(src, dst, work)
		} else {
			copyErr = cloneFile(src, dst, work, true)
		}
		if copyErr != nil {
			_ = os.RemoveAll(tmp)
			return copyErr
		}
	}
	if !treeHasFiles(tmp) {
		_ = os.RemoveAll(tmp)
		return nil
	}
	if err := os.WriteFile(filepath.Join(tmp, cacheMarker), []byte("ok\n"), 0o644); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.RemoveAll(final)
	if err := os.Rename(tmp, final); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	pruneCacheKind(parent, plan.fp)
	return nil
}

func treeHasFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() == cacheMarker {
			return nil
		}
		found = true
		return fs.SkipAll
	})
	return found
}

func restoreStepCache(entry, work string) error {
	if !cacheComplete(entry) {
		return fmt.Errorf("incomplete cache")
	}
	entries, err := os.ReadDir(entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == cacheMarker {
			continue
		}
		src := filepath.Join(entry, e.Name())
		dst := filepath.Join(work, e.Name())
		st, err := os.Lstat(src)
		if err != nil {
			return err
		}
		if st.IsDir() {
			if err := cloneTree(src, dst, entry); err != nil {
				return err
			}
			continue
		}
		if err := cloneFile(src, dst, entry, true); err != nil {
			return err
		}
	}
	return nil
}

func warmRestore(entry, work string, rels []string) int {
	n := 0
	for _, rel := range rels {
		src := filepath.Join(entry, rel)
		st, err := os.Lstat(src)
		if err != nil {
			continue
		}
		dst := filepath.Join(work, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			continue
		}
		var copyErr error
		if st.IsDir() {
			copyErr = cloneTree(src, dst, entry)
		} else {
			copyErr = cloneFile(src, dst, entry, true)
		}
		if copyErr == nil {
			n++
		}
	}
	return n
}

func chownRestored(entry, work string, cred *osuser.Credential) {
	entries, err := os.ReadDir(entry)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == cacheMarker {
			continue
		}
		_ = chownTree(filepath.Join(work, e.Name()), cred)
	}
}

// cloneTree mirrors src onto dst. Files are hardlinked so a node_modules restore
// does not copy tens of thousands of files. Absolute symlinks that point inside
// the source tree are rewritten relative, so they still resolve after the
// release id changes.
func cloneTree(src, dst, treeRoot string) error {
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
		if skipCachedRel(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return cloneFile(path, target, treeRoot, true)
	})
}

func skipCachedRel(rel string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range strings.Split(rel, "/") {
		if p == ".cache" {
			return true
		}
	}
	return false
}

func cloneFile(src, dst, treeRoot string, link bool) error {
	st, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return cloneSymlink(src, dst, treeRoot)
	}
	_ = os.Remove(dst)
	if link {
		if err := os.Link(src, dst); err == nil {
			return nil
		}
	}
	return copyFile(src, dst)
}

func cloneSymlink(src, dst, treeRoot string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if filepath.IsAbs(target) && treeRoot != "" {
		rel, err := filepath.Rel(treeRoot, target)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			if r, err := filepath.Rel(filepath.Dir(src), target); err == nil {
				target = r
			}
		}
	}
	_ = os.Remove(dst)
	return os.Symlink(target, dst)
}

func latestCacheEntry(base, appKey string, kinds ...string) string {
	var best string
	var bestTime time.Time
	for _, kind := range kinds {
		parent := filepath.Join(base, cacheDirName, "apps", appKey, kind)
		entries, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(parent, e.Name())
			if !cacheComplete(p) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if best == "" || info.ModTime().After(bestTime) {
				best = p
				bestTime = info.ModTime()
			}
		}
	}
	return best
}

func pruneCacheKind(parent, keep string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	type item struct {
		path string
		mod  time.Time
	}
	var items []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(parent, e.Name())
		if !cacheComplete(p) && e.Name() != keep {
			_ = os.RemoveAll(p)
			continue
		}
		items = append(items, item{path: p, mod: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	n := 0
	for _, it := range items {
		base := filepath.Base(it.path)
		if base == keep {
			n++
			continue
		}
		if n >= keepCacheEntries {
			_ = os.RemoveAll(it.path)
			continue
		}
		n++
	}
}

func toolchainEnv() map[string]string {
	// Prefer tarballs already on disk. Do not relocate GOCACHE, the npm cache,
	// or the pnpm store: those already live under the deploy user's home, and
	// a new empty directory would make the next changed build slower.
	return map[string]string{
		"npm_config_prefer_offline": "true",
		"npm_config_fund":           "false",
	}
}

func mergeToolchainEnv(env map[string]string, work string) map[string]string {
	base := projectBaseFromWork(work)
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	if base == "" {
		return out
	}
	for k, v := range toolchainEnv() {
		if _, ok := out[k]; ok {
			continue
		}
		out[k] = v
	}
	return out
}
