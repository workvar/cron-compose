package sourceupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoFromGitHubURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://github.com/workvar/cron-compose", "workvar/cron-compose"},
		{"https://github.com/workvar/cron-compose.git", "workvar/cron-compose"},
		{"https://github.com/workvar/cron-compose/releases/download/v1/croncompose-agent-linux-amd64", ""},
		{"https://example.com/workvar/cron-compose", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := repoFromGitHubURL(tc.in); got != tc.want {
			t.Errorf("repoFromGitHubURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsSource(t *testing.T) {
	if !IsSource("https://github.com/workvar/cron-compose", "") {
		t.Fatal("notes-only github url should be a source update")
	}
	if IsSource("https://github.com/workvar/cron-compose/releases/download/v1/bin", "aabb") {
		t.Fatal("release asset url should not be a source update")
	}
	if IsSource("https://github.com/workvar/cron-compose", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") {
		t.Fatal("checksummed url is a binary update")
	}
}

func TestFindStackRoot(t *testing.T) {
	dir := t.TempDir()
	if got := findStackRootAt(dir); got != "" {
		t.Fatalf("empty dir should not be a stack root, got %q", got)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ecosystem.config.js"), []byte("module.exports = { apps: [] }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findStackRootAt(dir); got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}

	nested := filepath.Join(dir, "agent", "bin")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findStackRootAt(nested); got != dir {
		t.Fatalf("walk-up got %q, want %q", got, dir)
	}
}
