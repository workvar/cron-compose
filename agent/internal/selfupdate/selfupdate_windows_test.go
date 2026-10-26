//go:build windows

package selfupdate

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// The Windows update path: the running .exe is renamed aside (Windows allows
// renaming, not deleting, an executable in use) and the new one takes its name.
func TestWindowsRenameSwapKeepsOld(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "croncompose-agent.exe")
	built := filepath.Join(t.TempDir(), "new.exe")
	if err := os.WriteFile(self, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(built, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hold the target open the way the image loader does for a running program:
	// shared for read and delete (rename), which is what makes the swap possible.
	name, _ := windows.UTF16PtrFromString(self)
	h, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)

	if _, err := installFileAt(built, self); err != nil {
		t.Fatalf("installFileAt: %v", err)
	}
	if got, _ := os.ReadFile(self); string(got) != "new-binary" {
		t.Fatalf("self = %q", got)
	}
	if got, _ := os.ReadFile(self + ".old"); string(got) != "old" {
		t.Fatalf("backup = %q", got)
	}
}
