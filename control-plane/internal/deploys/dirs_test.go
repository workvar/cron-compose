package deploys

import (
	"fmt"
	"testing"
)

func TestNormalizeRepoPath(t *testing.T) {
	if got := NormalizeRepoPath(" ./apps/web/ "); got != "apps/web" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRepoPath(""); got != "" {
		t.Fatalf("empty query path got %q", got)
	}
	if got := NormalizeRepoPath("."); got != "" {
		t.Fatalf(". as query parent should be empty, got %q", got)
	}
	if got := NormalizeRepoPath("apps//web"); got != "apps/web" {
		t.Fatalf("collapsed slashes got %q", got)
	}
	if got := NormalizeRepoPath("./apps"); got != "apps" {
		t.Fatalf("strip ./ got %q", got)
	}
}

func TestDirBaseName(t *testing.T) {
	cases := map[string]string{
		"":           ".",
		".":          ".",
		"apps":       "apps",
		"apps/web":   "web",
		"apps/web/":  "web",
	}
	for in, want := range cases {
		if got := DirBaseName(in); got != want {
			t.Errorf("DirBaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShouldSkipDirName(t *testing.T) {
	if !ShouldSkipDirName(".git") {
		t.Fatal(".git should be skipped")
	}
	if ShouldSkipDirName(".github") {
		t.Fatal(".github should not be skipped")
	}
	if ShouldSkipDirName("web") {
		t.Fatal("web should not be skipped")
	}
}

func TestFilterAndCapDirsShallow(t *testing.T) {
	items, trunc := FilterAndCapDirs([]string{"apps", "backend", ".git"}, "", false, 2000)
	if trunc || len(items) != 2 {
		t.Fatalf("items=%v trunc=%v", items, trunc)
	}
	if items[0].Path != "apps" || items[1].Path != "backend" {
		t.Fatalf("items=%v", items)
	}

	items, trunc = FilterAndCapDirs([]string{"apps/web", "apps/api", "apps/web/nested", "backend"}, "apps", false, 2000)
	if trunc || len(items) != 2 {
		t.Fatalf("shallow under apps: items=%v trunc=%v", items, trunc)
	}
}

func TestFilterAndCapDirsRecursivePrefixAndCap(t *testing.T) {
	var paths []string
	for i := 0; i < 5; i++ {
		paths = append(paths, fmt.Sprintf("apps/p%d", i))
	}
	items, trunc := FilterAndCapDirs(paths, "apps", true, 3)
	if !trunc || len(items) != 3 {
		t.Fatalf("len=%d trunc=%v", len(items), trunc)
	}
	for i, item := range items {
		if item.Name != fmt.Sprintf("p%d", i) {
			t.Fatalf("item[%d]=%+v", i, item)
		}
	}
}
