package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAppRootSkipsGitAndFlattens(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	web := filepath.Join(src, "web")
	if err := os.MkdirAll(filepath.Join(web, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "package.json"), []byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "src", "index.ts"), []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyAppRoot(web, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "package.json")); err != nil {
		t.Fatalf("package.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "src", "index.ts")); err != nil {
		t.Fatalf("src missing: %v", err)
	}

	full := t.TempDir()
	if err := copyAppRoot(src, full); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(full, ".git")); !os.IsNotExist(err) {
		t.Fatal(".git must be skipped when copying the repo root")
	}
	if _, err := os.Stat(filepath.Join(full, "web", "package.json")); err != nil {
		t.Fatalf("web/package.json missing: %v", err)
	}
}

func TestNormalizeRoot(t *testing.T) {
	if got := normalizeRoot(""); got != "." {
		t.Errorf("empty: %q", got)
	}
	if got := normalizeRoot("web"); got != "web" {
		t.Errorf("web: %q", got)
	}
	if got := normalizeRoot("/web"); got != "web" {
		t.Errorf("/web: %q", got)
	}
	if got := normalizeRoot("./web/app"); got != filepath.Join("web", "app") && got != "web/app" {
		// filepath.Clean on unix yields web/app
		if got != "web/app" {
			t.Errorf("./web/app: %q", got)
		}
	}
}
