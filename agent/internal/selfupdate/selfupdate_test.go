package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFileAtRenamesWhenDirWritable(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "croncompose-agent")
	built := filepath.Join(dir, "built")
	if err := os.WriteFile(self, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(built, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := installFileAt(built, self)
	if err != nil {
		t.Fatal(err)
	}
	if got != self {
		t.Fatalf("path=%q", got)
	}
	body, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new-binary" {
		t.Fatalf("body=%q", body)
	}
	bak, err := os.ReadFile(self + ".old")
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != "old" {
		t.Fatalf("backup=%q", bak)
	}
}

func TestInstallFileAtOverwritesWhenDirNotWritable(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "locked")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	self := filepath.Join(dir, "croncompose-agent")
	if err := os.WriteFile(self, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Drop directory write so creating <name>.new fails the way /usr/local/bin does.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	built := filepath.Join(t.TempDir(), "built")
	if err := os.WriteFile(built, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := installFileAt(built, self)
	if err != nil {
		t.Fatal(err)
	}
	if got != self {
		t.Fatalf("path=%q", got)
	}
	body, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new-binary" {
		t.Fatalf("body=%q", body)
	}
	bak, err := os.ReadFile(filepath.Join(dataDir, "croncompose-agent.old"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != "old" {
		t.Fatalf("backup=%q", bak)
	}
}

func TestDirWritable(t *testing.T) {
	dir := t.TempDir()
	if !dirWritable(dir) {
		t.Fatal("expected writable")
	}
	locked := filepath.Join(dir, "ro")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if dirWritable(locked) {
		t.Fatal("expected not writable")
	}
}
