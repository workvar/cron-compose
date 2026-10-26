//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPathMapsUnixPathsOntoSystemDrive(t *testing.T) {
	t.Setenv("SystemDrive", "D:")
	if got := LocalPath("/opt/apps/web"); got != `D:\opt\apps\web` {
		t.Fatalf("got %q", got)
	}
	if got := LocalPath(`E:\sites\web`); got != `E:\sites\web` {
		t.Fatalf("drive path changed: %q", got)
	}
	got := LocalPath("/tmp/x")
	if !strings.EqualFold(got, filepath.Join(os.TempDir(), "x")) {
		t.Fatalf("/tmp mapping = %q", got)
	}
}

func TestBashIsNeverTheWSLLauncher(t *testing.T) {
	if p := Bash(); isWSLLauncher(p) {
		t.Fatalf("Bash() returned the WSL launcher %q", p)
	}
}
