package deploys

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec is the croncompose.yml checked into an imported repo. CronCompose remains
// the source of truth; this file keeps GitHub Actions and the UI aligned.
type Spec struct {
	Name           string            `yaml:"name,omitempty" json:"name,omitempty"`
	Provider       string            `yaml:"provider" json:"provider"`
	Repo           string            `yaml:"repo" json:"repo"`
	Branch         string            `yaml:"branch,omitempty" json:"branch,omitempty"`
	Language       string            `yaml:"language,omitempty" json:"language,omitempty"`
	Install        string            `yaml:"install,omitempty" json:"install,omitempty"`
	Root           string            `yaml:"root,omitempty" json:"root,omitempty"`
	Port           int               `yaml:"port,omitempty" json:"port,omitempty"`
	ProcessManager string            `yaml:"process_manager,omitempty" json:"process_manager,omitempty"`
	ClonePath      string            `yaml:"clone_path,omitempty" json:"clone_path,omitempty"`
	Apps           []SpecApp         `yaml:"apps,omitempty" json:"apps,omitempty"`
	Env            map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// SpecApp is one package inside a monorepo.
type SpecApp struct {
	Name           string `yaml:"name" json:"name"`
	Root           string `yaml:"root" json:"root"`
	Language       string `yaml:"language,omitempty" json:"language,omitempty"`
	Install        string `yaml:"install,omitempty" json:"install,omitempty"`
	Port           int    `yaml:"port,omitempty" json:"port,omitempty"`
	ProcessManager string `yaml:"process_manager,omitempty" json:"process_manager,omitempty"`
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

// GitHubActionsWorkflow is the trigger-only workflow committed next to croncompose.yml.
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
            -d '{"trigger":"api"}'
`, url)) + "\n"
}

// GitLabCI is the trigger job committed as .gitlab-ci.yml.
func GitLabCI(publicBase, projectID string) string {
	base := strings.TrimRight(publicBase, "/")
	hook := fmt.Sprintf("%s/api/deploys/%s/runs", base, projectID)
	return strings.TrimSpace(fmt.Sprintf("deploy:\n  rules:\n    - if: $CI_COMMIT_BRANCH\n  script:\n    - curl -fsS -X POST %q -H \"Authorization: Bearer $CRONCOMPOSE_TOKEN\" -H \"content-type: application/json\" -d '{\"trigger\":\"api\"}'\n", hook)) + "\n"
}

func specFiles(p Project, publicBase string) []RepoFile {
	raw, _ := MarshalSpec(Spec{
		Name: p.Name, Provider: p.Provider, Repo: p.RepoFullName, Branch: p.DefaultBranch,
		Language: p.Language, Install: p.InstallScript, Root: p.RootDirectory,
		Port: p.Port, ProcessManager: p.ProcessManager, ClonePath: p.ClonePath,
		Apps: p.Apps, Env: p.Env,
	})
	files := []RepoFile{{Path: "croncompose.yml", Content: string(raw)}}
	if p.Provider == "gitlab" {
		files = append(files, RepoFile{Path: ".gitlab-ci.yml", Content: GitLabCI(publicBase, p.ID)})
	} else {
		files = append(files, RepoFile{Path: ".github/workflows/croncompose.yml", Content: GitHubActionsWorkflow(publicBase, p.ID)})
	}
	return files
}
