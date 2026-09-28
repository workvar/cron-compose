package deploys

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec is the croncompose.yml checked into an imported repo. CronCompose remains
// the source of truth; this file keeps GitHub Actions and the UI aligned.
//
// It is also an input: the importer reads it from the repo root (or from a pasted
// file) and pre-fills the deploy with it. See ParseSpecFile and the public /docs page.
type Spec struct {
	Version  int    `yaml:"version,omitempty" json:"version,omitempty"`
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Provider string `yaml:"provider" json:"provider"`
	Repo     string `yaml:"repo" json:"repo"`
	Branch   string `yaml:"branch,omitempty" json:"branch,omitempty"`
	// Server is a server name or id; the importer preselects it when it matches.
	Server         string            `yaml:"server,omitempty" json:"server,omitempty"`
	Language       string            `yaml:"language,omitempty" json:"language,omitempty"`
	Install        string            `yaml:"install,omitempty" json:"install,omitempty"`
	Root           string            `yaml:"root,omitempty" json:"root,omitempty"`
	Port           int               `yaml:"port,omitempty" json:"port,omitempty"`
	ProcessManager string            `yaml:"process_manager,omitempty" json:"process_manager,omitempty"`
	ClonePath      string            `yaml:"clone_path,omitempty" json:"clone_path,omitempty"`
	Apps           []SpecApp         `yaml:"apps,omitempty" json:"apps,omitempty"`
	Env            map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Health         *SpecHealth       `yaml:"health,omitempty" json:"health,omitempty"`
	DeployTimeout  int               `yaml:"deploy_timeout,omitempty" json:"deploy_timeout,omitempty"`
	AutoRollback   bool              `yaml:"auto_rollback,omitempty" json:"auto_rollback,omitempty"`
}

// SpecHealth is the optional post-deploy HTTP probe.
type SpecHealth struct {
	Path    string `yaml:"path" json:"path"`
	Port    int    `yaml:"port,omitempty" json:"port,omitempty"`
	Timeout int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// SpecApp is one package inside a monorepo.
type SpecApp struct {
	Name           string   `yaml:"name" json:"name"`
	Root           string   `yaml:"root" json:"root"`
	Language       string   `yaml:"language,omitempty" json:"language,omitempty"`
	Install        string   `yaml:"install,omitempty" json:"install,omitempty"`
	Port           int      `yaml:"port,omitempty" json:"port,omitempty"`
	ProcessManager string   `yaml:"process_manager,omitempty" json:"process_manager,omitempty"`
	Env            []EnvVar `yaml:"env,omitempty" json:"env,omitempty"`
	// Health overrides the project's shared health check for this one app. Unset
	// (nil, or an empty Path) falls back to the project-level Health below, so an
	// app that doesn't set its own keeps behaving exactly as it did before this
	// existed.
	Health *SpecHealth `yaml:"health,omitempty" json:"health,omitempty"`
}

// MarshalSpec renders croncompose.yml.
func MarshalSpec(s Spec) ([]byte, error) {
	return yaml.Marshal(s)
}

// UnmarshalSpec parses croncompose.yml.
func UnmarshalSpec(raw []byte) (Spec, error) {
	var s Spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return Spec{}, err
	}
	return s, nil
}

// GitHubActionsWorkflow is the trigger-only workflow committed next to
// croncompose.yml. It builds nothing: the build happens on the target server, so the
// job's whole purpose is to tell CronCompose that a commit landed. It reports the
// branch and commit explicitly, so a push to a feature branch does not redeploy the
// default branch, and so the control plane can recognize this trigger and the push
// webhook as the same event and deploy once.
func GitHubActionsWorkflow(publicBase, projectID string) string {
	base := strings.TrimRight(publicBase, "/")
	url := fmt.Sprintf("%s/api/deploys/%s/runs", base, projectID)
	return strings.TrimSpace(fmt.Sprintf(`
name: Deploy to CronCompose
on:
  push:
    branches: ["**"]
  workflow_dispatch:
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger CronCompose
        env:
          CRONCOMPOSE_TOKEN: ${{ secrets.CRONCOMPOSE_TOKEN }}
        run: |
          curl -fsS -X POST %q \
            -H "Authorization: Bearer $CRONCOMPOSE_TOKEN" \
            -H "content-type: application/json" \
            -d "{\"trigger\":\"api\",\"branch\":\"${{ github.ref_name }}\",\"commit\":\"${{ github.sha }}\"}"
`, url)) + "\n"
}

// GitLabCI is the trigger job committed as .gitlab-ci.yml. Same shape as the GitHub
// workflow above, including reporting the branch and commit it is triggering for.
func GitLabCI(publicBase, projectID string) string {
	base := strings.TrimRight(publicBase, "/")
	hook := fmt.Sprintf("%s/api/deploys/%s/runs", base, projectID)
	body := `{"trigger":"api","branch":"$CI_COMMIT_REF_NAME","commit":"$CI_COMMIT_SHA"}`
	return fmt.Sprintf("deploy:\n  rules:\n    - if: $CI_COMMIT_BRANCH\n  script:\n    - curl -fsS -X POST %q -H \"Authorization: Bearer $CRONCOMPOSE_TOKEN\" -H \"content-type: application/json\" -d '%s'\n", hook, body)
}

// SpecForProject renders a project back into the croncompose.yml shape.
func SpecForProject(p Project) Spec {
	s := Spec{
		Version: SpecVersion, Name: p.Name, Provider: p.Provider, Repo: p.RepoFullName, Branch: p.DefaultBranch,
		Language: p.Language, Install: p.InstallScript, Root: p.RootDirectory,
		Port: p.Port, ProcessManager: p.ProcessManager, ClonePath: p.ClonePath,
		Apps: appsForSpec(p.Apps), Env: p.Env,
		DeployTimeout: p.DeployTimeoutSeconds, AutoRollback: p.AutoRollback,
	}
	if p.HealthPath != "" {
		s.Health = &SpecHealth{Path: p.HealthPath, Port: p.HealthPort, Timeout: p.HealthTimeoutSeconds}
	}
	return s
}

// specFiles is what provisionRemote commits. keepSpec leaves an existing
// croncompose.yml alone: the project was imported from it, so it is the user's file.
func specFiles(p Project, publicBase string, keepSpec bool) []RepoFile {
	var files []RepoFile
	if !keepSpec {
		raw, _ := MarshalSpec(SpecForProject(p))
		files = append(files, RepoFile{Path: "croncompose.yml", Content: string(raw)})
	}
	if p.Provider == "gitlab" {
		files = append(files, RepoFile{Path: ".gitlab-ci.yml", Content: GitLabCI(publicBase, p.ID)})
	} else {
		files = append(files, RepoFile{Path: ".github/workflows/croncompose.yml", Content: GitHubActionsWorkflow(publicBase, p.ID)})
	}
	return files
}

// appsForSpec drops secrets/ciphertext so croncompose.yml never commits them.
func appsForSpec(apps []SpecApp) []SpecApp {
	out := make([]SpecApp, len(apps))
	for i, a := range apps {
		out[i] = a
		var env []EnvVar
		for _, v := range a.Env {
			if v.Sensitive || v.Key == "" {
				continue
			}
			env = append(env, EnvVar{Key: v.Key, Value: v.Value})
		}
		out[i].Env = env
	}
	return out
}
