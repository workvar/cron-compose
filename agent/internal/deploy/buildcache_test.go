package deploy

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitCacheableKeepsMixedScripts(t *testing.T) {
	inst, build, ok := splitCacheable("npm ci && npm run build", "nextjs")
	if !ok || inst != "npm ci" || build != "npm run build" {
		t.Fatalf("split = %q %q ok=%v", inst, build, ok)
	}
	if _, _, ok := splitCacheable("go build -o app . && cp app /tmp/app", "go"); ok {
		t.Fatal("a copy after the compile must stay one script")
	}
	if _, _, ok := splitCacheable("make prod", "node"); ok {
		t.Fatal("unrecognized install must not be cached as a split")
	}
}

func TestInstallFingerprintIgnoresSource(t *testing.T) {
	work := filepath.Join(t.TempDir(), "releases", "r1", "web")
	writeFile(t, filepath.Join(work, "package.json"), `{"name":"web"}`)
	writeFile(t, filepath.Join(work, "package-lock.json"), "lock-a")
	writeFile(t, filepath.Join(work, "index.js"), "v1")

	job := appScript{
		app: "Frontend", flowPhase: "install", work: work,
		script: "npm ci", language: "nextjs",
	}
	install, ok := planStepCache(job)
	if !ok {
		t.Fatal("expected an install plan")
	}
	buildJob := job
	buildJob.flowPhase = "build"
	buildJob.script = "npm run build"
	build, ok := planStepCache(buildJob)
	if !ok {
		t.Fatal("expected a build plan")
	}
	writeFile(t, filepath.Join(work, "index.js"), "v2")
	install2, _ := planStepCache(job)
	build2, _ := planStepCache(buildJob)
	if install.fp != install2.fp {
		t.Fatal("source change must not bust the install cache")
	}
	if build.fp == build2.fp {
		t.Fatal("source change must bust the build cache")
	}
	writeFile(t, filepath.Join(work, "package-lock.json"), "lock-b")
	install3, _ := planStepCache(job)
	if install.fp == install3.fp {
		t.Fatal("lockfile change must bust the install cache")
	}
}

func TestInstallCacheRoundTripSkipsPackageCache(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "releases", "r1", "web")
	writeFile(t, filepath.Join(work, "package.json"), `{"name":"web"}`)
	writeFile(t, filepath.Join(work, "package-lock.json"), "lock-a")
	writeFile(t, filepath.Join(work, "node_modules", "pkg", "index.js"), "hello")
	writeFile(t, filepath.Join(work, "node_modules", ".cache", "junk"), "nope")
	// Absolute bin links must survive a new release id.
	target := filepath.Join(work, "node_modules", "pkg", "index.js")
	link := filepath.Join(work, "node_modules", ".bin", "tool")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	job := appScript{
		app: "Frontend", flowPhase: "install", work: work,
		script: "npm ci", language: "nextjs",
	}
	plan, ok := planStepCache(job)
	if !ok {
		t.Fatal("expected a plan")
	}
	if err := saveStepCache(plan, work, job.language, job.script); err != nil {
		t.Fatal(err)
	}

	work2 := filepath.Join(base, "releases", "r2", "web")
	writeFile(t, filepath.Join(work2, "package.json"), `{"name":"web"}`)
	writeFile(t, filepath.Join(work2, "package-lock.json"), "lock-a")
	var logged string
	m := NewManager(slog.Default(), func(ev *agentv1.DeployEvent) {
		logged += string(ev.Data)
	})
	job.work = work2
	plan2, ok := planStepCache(job)
	if !ok || plan2.fp != plan.fp {
		t.Fatalf("fingerprint changed across releases")
	}
	if !m.replayCachedStep("run", "", phaseInstall, work2, plan2, nil) {
		t.Fatal("expected a cache hit")
	}
	body, err := os.ReadFile(filepath.Join(work2, "node_modules", "pkg", "index.js"))
	if err != nil || string(body) != "hello" {
		t.Fatalf("restored module = %q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(work2, "node_modules", ".cache", "junk")); !os.IsNotExist(err) {
		t.Fatal("node_modules/.cache must not be cached")
	}
	got, err := os.Readlink(filepath.Join(work2, "node_modules", ".bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(got) {
		t.Fatalf("symlink stayed absolute: %s", got)
	}
	resolved := filepath.Join(filepath.Dir(filepath.Join(work2, "node_modules", ".bin", "tool")), got)
	if filepath.Clean(resolved) != filepath.Join(work2, "node_modules", "pkg", "index.js") {
		t.Fatalf("symlink %s resolved to %s", got, resolved)
	}
	if !strings.Contains(logged, "cache hit") {
		t.Fatalf("log = %q", logged)
	}
}

func TestBuildCacheRestoresBinaryAndWarmsCompiler(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "releases", "r1", "proxy")
	writeFile(t, filepath.Join(work, "go.mod"), "module example.com/proxy\n\ngo 1.25\n")
	writeFile(t, filepath.Join(work, "main.go"), "package main\n")

	job := appScript{
		app: "Proxy", flowPhase: "build", work: work,
		script: "go build -o proxy .", language: "nextjs",
	}
	// Fingerprint is taken before the compiler writes its outputs, matching a deploy.
	plan, ok := planStepCache(job)
	if !ok {
		t.Fatal("expected a build plan")
	}
	writeFile(t, filepath.Join(work, "proxy"), "bin-v1")
	writeFile(t, filepath.Join(work, ".next", "BUILD_ID"), "id")
	writeFile(t, filepath.Join(work, ".next", "cache", "webpack", "x"), "warm")
	if err := saveStepCache(plan, work, job.language, job.script); err != nil {
		t.Fatal(err)
	}

	work2 := filepath.Join(base, "releases", "r2", "proxy")
	writeFile(t, filepath.Join(work2, "go.mod"), "module example.com/proxy\n\ngo 1.25\n")
	writeFile(t, filepath.Join(work2, "main.go"), "package main\nfunc changed() {}\n")
	job.work = work2
	plan2, ok := planStepCache(job)
	if !ok {
		t.Fatal("expected a plan")
	}
	if plan2.fp == plan.fp {
		t.Fatal("edited source should miss")
	}
	m := NewManager(slog.Default(), func(*agentv1.DeployEvent) {})
	m.warmCachedStep("run", "", work2, plan2)
	warm, err := os.ReadFile(filepath.Join(work2, ".next", "cache", "webpack", "x"))
	if err != nil || string(warm) != "warm" {
		t.Fatalf("compiler cache = %q err=%v", warm, err)
	}
	if _, err := os.Stat(filepath.Join(work2, ".next", "BUILD_ID")); !os.IsNotExist(err) {
		t.Fatal("a miss must not restore the previous build output")
	}

	// Same source as the cached release restores the binary and skips the compile.
	work3 := filepath.Join(base, "releases", "r3", "proxy")
	writeFile(t, filepath.Join(work3, "go.mod"), "module example.com/proxy\n\ngo 1.25\n")
	writeFile(t, filepath.Join(work3, "main.go"), "package main\n")
	job.work = work3
	plan3, ok := planStepCache(job)
	if !ok || plan3.fp != plan.fp {
		t.Fatal("unchanged source should hit")
	}
	if !m.replayCachedStep("run", "", phaseInstall, work3, plan3, nil) {
		t.Fatal("expected a build cache hit")
	}
	bin, err := os.ReadFile(filepath.Join(work3, "proxy"))
	if err != nil || string(bin) != "bin-v1" {
		t.Fatalf("binary = %q err=%v", bin, err)
	}
}

func TestCacheKeepsTwoEntries(t *testing.T) {
	base := t.TempDir()
	var fps []string
	for i, lock := range []string{"a", "b", "c"} {
		work := filepath.Join(base, "releases", string(rune('1'+i)), "web")
		writeFile(t, filepath.Join(work, "package.json"), `{"name":"web"}`)
		writeFile(t, filepath.Join(work, "package-lock.json"), lock)
		writeFile(t, filepath.Join(work, "node_modules", "pkg", "index.js"), lock)
		job := appScript{
			app: "Frontend", flowPhase: "install", work: work,
			script: "npm ci", language: "nextjs",
		}
		plan, ok := planStepCache(job)
		if !ok {
			t.Fatal("plan")
		}
		if err := saveStepCache(plan, work, job.language, job.script); err != nil {
			t.Fatal(err)
		}
		fps = append(fps, plan.fp)
		// Directory mtimes can share a coarse timestamp; keep prune order stable.
		time.Sleep(20 * time.Millisecond)
	}
	parent := filepath.Join(base, cacheDirName, "apps", "Frontend-web", "install")
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != keepCacheEntries {
		t.Fatalf("entries = %d, want %d", len(entries), keepCacheEntries)
	}
	if _, err := os.Stat(filepath.Join(parent, fps[0])); !os.IsNotExist(err) {
		t.Fatal("oldest fingerprint should have been pruned")
	}
	if _, err := os.Stat(filepath.Join(parent, fps[2])); err != nil {
		t.Fatal("newest fingerprint missing", err)
	}
}

func TestToolchainEnvDoesNotOverrideApp(t *testing.T) {
	work := filepath.Join(t.TempDir(), "releases", "r1", "web")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	got := mergeToolchainEnv(map[string]string{"GOCACHE": "/keep", "PORT": "3000"}, work)
	if got["GOCACHE"] != "/keep" {
		t.Fatalf("GOCACHE = %s", got["GOCACHE"])
	}
	if got["PORT"] != "3000" {
		t.Fatalf("PORT = %s", got["PORT"])
	}
	if _, ok := got["GOMODCACHE"]; ok {
		t.Fatal("module cache must stay on the deploy user's existing path")
	}
	if got["npm_config_prefer_offline"] != "true" {
		t.Fatalf("prefer offline = %s", got["npm_config_prefer_offline"])
	}
}

func TestProjectBaseIgnoresAppNamedReleases(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "releases", "20260101T000000Z-abc", "releases")
	if got := projectBaseFromWork(work); got != base {
		t.Fatalf("base = %s, want %s", got, base)
	}
	if projectBaseFromWork(t.TempDir()) != "" {
		t.Fatal("a directory outside the release layout has no cache root")
	}
}

func TestUnrecognizedScriptIsNotCached(t *testing.T) {
	work := filepath.Join(t.TempDir(), "releases", "r1", "web")
	writeFile(t, filepath.Join(work, "package.json"), `{"name":"web"}`)
	job := appScript{
		app: "Frontend", flowPhase: "install", work: work,
		script: "make prod", language: "node",
	}
	if _, ok := planStepCache(job); ok {
		t.Fatal("make prod must run every time")
	}
}
