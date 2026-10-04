package deploys

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsSpecFileName(t *testing.T) {
	cases := map[string]bool{
		"croncompose.yml":           true,
		"croncompose.yaml":          true,
		".croncompose.yml":          true,
		"apps/web/croncompose.yml":  true,
		"apps/web/croncompose.yaml": true,
		"package.json":              false,
		"croncompose.yml.bak":       false,
		"apps/web/Dockerfile":       false,
	}
	for path, want := range cases {
		if got := IsSpecFileName(path); got != want {
			t.Fatalf("%q: got %v want %v", path, got, want)
		}
	}
}

func TestListSpecFilesGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tree": []map[string]string{
					{"path": "croncompose.yml", "type": "blob"},
					{"path": "apps/web/croncompose.yml", "type": "blob"},
					{"path": "apps/web/package.json", "type": "blob"},
					{"path": "apps/api", "type": "tree"},
					{"path": "docs/croncompose.yml.bak", "type": "blob"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	items, err := g.ListSpecFiles(context.Background(), "github", "tok", "acme/r", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Path != "croncompose.yml" || items[1].Path != "apps/web/croncompose.yml" {
		t.Fatalf("%+v", items)
	}
}
