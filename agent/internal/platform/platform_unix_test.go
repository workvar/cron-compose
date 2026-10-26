//go:build !windows

package platform

import "testing"

func TestUnixDefaultsAreUnchanged(t *testing.T) {
	if Bash() != "/bin/bash" || TempDir() != "/tmp" {
		t.Fatalf("bash=%q tmp=%q", Bash(), TempDir())
	}
	if LocalPath("/opt/apps/web") != "/opt/apps/web" {
		t.Fatal("LocalPath must not rewrite Unix paths")
	}
	if !HasBash() {
		t.Fatal("HasBash should be true on Unix")
	}
	if free, err := FreeBytes("/"); err != nil || free == 0 {
		t.Fatalf("FreeBytes(/) = %d, %v", free, err)
	}
}
