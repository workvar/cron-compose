package deploys

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnsureWebhookGitHub(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	if err := g.EnsureWebhook(context.Background(), "github", "tok", "acme/web", "",
		"https://cron.example.com/api/deploys/webhooks/github", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/repos/acme/web/hooks" {
		t.Fatalf("github hook %s %s", method, path)
	}
	if !strings.Contains(body, `"url":"https://cron.example.com/api/deploys/webhooks/github"`) {
		t.Errorf("body = %s", body)
	}
	if !strings.Contains(body, `"secret":"s3cret"`) {
		t.Errorf("missing secret in %s", body)
	}
}

func TestEnsureWebhookGitHubAlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Hook already exists"}`))
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	if err := g.EnsureWebhook(context.Background(), "github", "tok", "acme/web", "",
		"https://cron.example.com/api/deploys/webhooks/github", "s"); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRepoFilesGitLab(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()
	g := NewGitAPI(srv.URL)
	g.http = srv.Client()
	files := []RepoFile{
		{Path: "croncompose.yml", Content: "provider: gitlab\n"},
		{Path: ".github/workflows/croncompose.yml", Content: "name: Deploy\n"},
	}
	if err := g.EnsureRepoFiles(context.Background(), "gitlab", "tok", "acme/web", "42", "main", files); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || !strings.Contains(path, "/projects/") || !strings.HasSuffix(path, "/repository/commits") {
		t.Fatalf("gitlab commit %s %s", method, path)
	}
	var payload struct {
		Branch  string `json:"branch"`
		Actions []struct {
			Action   string `json:"action"`
			FilePath string `json:"file_path"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Branch != "main" || len(payload.Actions) != 2 {
		t.Errorf("payload = %s", body)
	}
}

func TestApplyUpdate(t *testing.T) {
	p := Project{Name: "old", ProcessManager: "none", Port: 0, InstallScript: "npm i"}
	pm := "pm2"
	port := 3000
	got := applyUpdate(p, UpdateInput{ProcessManager: &pm, Port: &port})
	if got.ProcessManager != "pm2" || got.Port != 3000 || got.Name != "old" || got.InstallScript != "npm i" {
		t.Errorf("got %+v", got)
	}
}
