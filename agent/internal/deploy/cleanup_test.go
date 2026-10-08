package deploy

import (
	"strings"
	"testing"
)

func TestDefaultCleanupNext(t *testing.T) {
	got := defaultCleanup("nextjs")
	if got == "" || !strings.Contains(got, ".next/cache") || !strings.Contains(got, "node_modules/.cache") {
		t.Fatalf("unexpected nextjs cleanup: %q", got)
	}
}

func TestCleanupScriptForOverride(t *testing.T) {
	if got := cleanupScriptFor("rm -rf dist", "nextjs"); got != "rm -rf dist" {
		t.Fatalf("got %q", got)
	}
	if got := cleanupScriptFor("", "go"); got != defaultCleanup("go") {
		t.Fatalf("got %q", got)
	}
}
