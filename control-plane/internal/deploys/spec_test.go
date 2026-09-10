package deploys

import (
	"strings"
	"testing"
)

func TestSpecRoundTrip(t *testing.T) {
	in := Spec{
		Name:           "web",
		Provider:       "github",
		Repo:           "acme/web",
		Branch:         "main",
		Language:       "node",
		Install:        "npm install && npm run build",
		Root:           ".",
		Port:           3000,
		ProcessManager: "pm2",
		ClonePath:      "/opt/apps/node/web",
		Apps: []SpecApp{
			{Name: "web", Root: "apps/web", Language: "node", Install: "pnpm install", Port: 3000},
		},
		Env: map[string]string{"NODE_ENV": "production"},
	}
	raw, err := MarshalSpec(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "provider: github") {
		t.Fatalf("yaml missing provider:\n%s", raw)
	}
	got, err := UnmarshalSpec(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != in.Repo || got.Port != 3000 || len(got.Apps) != 1 {
		t.Errorf("round trip = %+v", got)
	}
}

func TestGitHubActionsWorkflow(t *testing.T) {
	yml := GitHubActionsWorkflow("https://cron.example.com", "proj_1")
	if !strings.Contains(yml, "cron.example.com/api/deploys/proj_1/runs") {
		t.Errorf("workflow missing trigger URL:\n%s", yml)
	}
	if !strings.Contains(yml, "CRONCOMPOSE_TOKEN") {
		t.Error("workflow missing token secret")
	}
}

func TestGitLabCI(t *testing.T) {
	yml := GitLabCI("https://cron.example.com", "proj_1")
	if !strings.Contains(yml, "cron.example.com/api/deploys/proj_1/runs") {
		t.Errorf("ci missing URL:\n%s", yml)
	}
}

func TestSpecFiles(t *testing.T) {
	gh := specFiles(Project{ID: "p1", Provider: "github", RepoFullName: "acme/web"}, "https://cron.example.com")
	if len(gh) != 2 || gh[0].Path != "croncompose.yml" || gh[1].Path != ".github/workflows/croncompose.yml" {
		t.Fatalf("github files = %+v", gh)
	}
	gl := specFiles(Project{ID: "p1", Provider: "gitlab", RepoFullName: "acme/web"}, "https://cron.example.com")
	if len(gl) != 2 || gl[1].Path != ".gitlab-ci.yml" {
		t.Fatalf("gitlab files = %+v", gl)
	}
}
