package deploys

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListDirsGitHubShallow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/contents") {
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "web", "path": "apps/web", "type": "dir"},
				{"name": "readme.md", "path": "apps/readme.md", "type": "file"},
				{"name": ".git", "path": "apps/.git", "type": "dir"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	out, err := g.ListDirs(context.Background(), "github", "tok", "acme/r", "main", "apps", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Recursive || out.Truncated || len(out.Items) != 1 || out.Items[0].Path != "apps/web" {
		t.Fatalf("%+v", out)
	}
	if out.Path != "apps" {
		t.Fatalf("path=%q", out.Path)
	}
}

func TestListDirsGitHubRecursiveCap(t *testing.T) {
	var tree []map[string]string
	for i := 0; i < MaxDirEntries+5; i++ {
		tree = append(tree, map[string]string{
			"path": fmt.Sprintf("apps/p%d", i),
			"type": "tree",
		})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"tree": tree})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	out, err := g.ListDirs(context.Background(), "github", "tok", "acme/r", "main", "apps", true)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Recursive || !out.Truncated || len(out.Items) != MaxDirEntries {
		t.Fatalf("recursive=%v truncated=%v len=%d", out.Recursive, out.Truncated, len(out.Items))
	}
}

func TestListDirsGitHubDefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/r" {
			_ = json.NewEncoder(w).Encode(map[string]string{"default_branch": "develop"})
			return
		}
		if strings.Contains(r.URL.Path, "/contents") {
			if r.URL.Query().Get("ref") != "develop" {
				http.Error(w, "wrong ref", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "src", "path": "src", "type": "dir"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	out, err := g.ListDirs(context.Background(), "github", "tok", "acme/r", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Path != "src" {
		t.Fatalf("%+v", out)
	}
}

func TestListDirsGitLabShallow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/tree") && r.URL.Query().Get("recursive") != "true" {
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "web", "path": "apps/web", "type": "tree"},
				{"name": "readme.md", "path": "apps/readme.md", "type": "blob"},
				{"name": ".git", "path": "apps/.git", "type": "tree"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI(srv.URL)
	g.http = srv.Client()
	out, err := g.ListDirs(context.Background(), "gitlab", "tok", "acme/r", "main", "apps", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Recursive || out.Truncated || len(out.Items) != 1 || out.Items[0].Path != "apps/web" {
		t.Fatalf("%+v", out)
	}
}

func TestListDirsGitLabRecursive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/tree") && r.URL.Query().Get("recursive") == "true" {
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "web", "path": "apps/web", "type": "tree"},
				{"name": "api", "path": "apps/api", "type": "tree"},
				{"name": "nested", "path": "apps/web/nested", "type": "tree"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI(srv.URL)
	g.http = srv.Client()
	out, err := g.ListDirs(context.Background(), "gitlab", "tok", "acme/r", "main", "apps", true)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Recursive || out.Truncated || len(out.Items) != 3 {
		t.Fatalf("%+v", out)
	}
}
