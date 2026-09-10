package deploys

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitAPI talks to GitHub or GitLab with the user's git grant.
type GitAPI struct {
	http       *http.Client
	gitlabBase string
	githubBase string // empty means https://api.github.com; tests override
}

// NewGitAPI returns a client with a 25s timeout. gitlabBase is the GitLab origin
// (https://gitlab.com or a self-hosted instance).
func NewGitAPI(gitlabBase string) *GitAPI {
	gitlabBase = strings.TrimRight(gitlabBase, "/")
	if gitlabBase == "" {
		gitlabBase = "https://gitlab.com"
	}
	return &GitAPI{http: &http.Client{Timeout: 25 * time.Second}, gitlabBase: gitlabBase}
}

func (g *GitAPI) gitlabAPI() string { return g.gitlabBase + "/api/v4" }

func (g *GitAPI) githubAPI() string {
	if g.githubBase != "" {
		return strings.TrimRight(g.githubBase, "/")
	}
	return "https://api.github.com"
}

// ListRepos lists repositories the token can see.
func (g *GitAPI) ListRepos(ctx context.Context, provider, token string) ([]Repo, error) {
	if provider == "gitlab" {
		return g.listGitLab(ctx, token)
	}
	return g.listGitHub(ctx, token)
}

func (g *GitAPI) listGitHub(ctx context.Context, token string) ([]Repo, error) {
	var raw []struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		CloneURL      string `json:"clone_url"`
		Private       bool   `json:"private"`
	}
	u := "https://api.github.com/user/repos?per_page=100&sort=updated&affiliation=owner,collaborator,organization_member"
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return nil, err
	}
	out := make([]Repo, 0, len(raw))
	for _, r := range raw {
		out = append(out, Repo{
			ID: fmt.Sprintf("%d", r.ID), FullName: r.FullName, Description: r.Description,
			DefaultBranch: r.DefaultBranch, CloneURL: r.CloneURL, Private: r.Private,
		})
	}
	return out, nil
}

func (g *GitAPI) listGitLab(ctx context.Context, token string) ([]Repo, error) {
	var raw []struct {
		ID                int64  `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		Description       string `json:"description"`
		DefaultBranch     string `json:"default_branch"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		Visibility        string `json:"visibility"`
	}
	u := g.gitlabAPI() + "/projects?membership=true&simple=true&per_page=100&order_by=updated_at"
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return nil, err
	}
	out := make([]Repo, 0, len(raw))
	for _, r := range raw {
		out = append(out, Repo{
			ID: fmt.Sprintf("%d", r.ID), FullName: r.PathWithNamespace, Description: r.Description,
			DefaultBranch: r.DefaultBranch, CloneURL: r.HTTPURLToRepo, Private: r.Visibility != "public",
		})
	}
	return out, nil
}

// FetchFiles downloads a subset of the repo tree for Detect().
func (g *GitAPI) FetchFiles(ctx context.Context, provider, token, fullName, branch string) (map[string]string, Repo, error) {
	if provider == "gitlab" {
		return g.fetchGitLab(ctx, token, fullName, branch)
	}
	return g.fetchGitHub(ctx, token, fullName, branch)
}

func (g *GitAPI) fetchGitHub(ctx context.Context, token, fullName, branch string) (map[string]string, Repo, error) {
	var meta struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
		CloneURL      string `json:"clone_url"`
		Private       bool   `json:"private"`
		Description   string `json:"description"`
	}
	if err := g.getJSON(ctx, "https://api.github.com/repos/"+fullName, token, &meta); err != nil {
		return nil, Repo{}, err
	}
	if branch == "" {
		branch = meta.DefaultBranch
	}
	repo := Repo{
		ID: fmt.Sprintf("%d", meta.ID), FullName: meta.FullName, Description: meta.Description,
		DefaultBranch: meta.DefaultBranch, CloneURL: meta.CloneURL, Private: meta.Private,
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", fullName, url.PathEscape(branch))
	_ = g.getJSON(ctx, treeURL, token, &tree)
	files := map[string]string{}
	n := 0
	for _, t := range tree.Tree {
		if t.Type != "blob" || !interestingPath(t.Path) {
			continue
		}
		body, err := g.githubFile(ctx, token, fullName, t.Path, branch)
		if err != nil {
			continue
		}
		files[t.Path] = body
		n++
		if n >= 40 {
			break
		}
	}
	if len(files) == 0 {
		// Root listing fallback.
		var entries []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		_ = g.getJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/contents/?ref=%s", fullName, url.QueryEscape(branch)), token, &entries)
		for _, e := range entries {
			if e.Type != "file" {
				continue
			}
			body, err := g.githubFile(ctx, token, fullName, e.Path, branch)
			if err != nil {
				continue
			}
			files[e.Path] = body
		}
	}
	return files, repo, nil
}

func (g *GitAPI) githubFile(ctx context.Context, token, fullName, path, branch string) (string, error) {
	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	u := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s", fullName, path, url.QueryEscape(branch))
	if err := g.getJSON(ctx, u, token, &file); err != nil {
		return "", err
	}
	if file.Encoding == "base64" {
		b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return file.Content, nil
}

func (g *GitAPI) fetchGitLab(ctx context.Context, token, fullName, branch string) (map[string]string, Repo, error) {
	enc := url.PathEscape(fullName)
	var meta struct {
		ID            int64  `json:"id"`
		Path          string `json:"path_with_namespace"`
		DefaultBranch string `json:"default_branch"`
		HTTPURL       string `json:"http_url_to_repo"`
		Visibility    string `json:"visibility"`
		Description   string `json:"description"`
	}
	if err := g.getJSON(ctx, g.gitlabAPI()+"/projects/"+enc, token, &meta); err != nil {
		return nil, Repo{}, err
	}
	if branch == "" {
		branch = meta.DefaultBranch
	}
	repo := Repo{
		ID: fmt.Sprintf("%d", meta.ID), FullName: meta.Path, Description: meta.Description,
		DefaultBranch: meta.DefaultBranch, CloneURL: meta.HTTPURL, Private: meta.Visibility != "public",
	}
	var tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	_ = g.getJSON(ctx, fmt.Sprintf("%s/projects/%s/repository/tree?recursive=true&per_page=100&ref=%s", g.gitlabAPI(), enc, url.QueryEscape(branch)), token, &tree)
	files := map[string]string{}
	n := 0
	for _, t := range tree {
		if t.Type != "blob" || !interestingPath(t.Path) {
			continue
		}
		u := fmt.Sprintf("%s/projects/%s/repository/files/%s/raw?ref=%s", g.gitlabAPI(), enc, url.PathEscape(t.Path), url.QueryEscape(branch))
		body, err := g.getRaw(ctx, u, token)
		if err != nil {
			continue
		}
		files[t.Path] = body
		n++
		if n >= 40 {
			break
		}
	}
	return files, repo, nil
}

func interestingPath(p string) bool {
	base := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i+1:]
	}
	switch base {
	case "package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", "yarn.lock", "bun.lockb", "bun.lock",
		"go.mod", "requirements.txt", "pyproject.toml", "Pipfile", "Cargo.toml", "Gemfile",
		"composer.json", "mix.exs", "pom.xml", "build.gradle", "build.gradle.kts",
		"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
		"ecosystem.config.js", "ecosystem.config.cjs", "ecosystem.config.mjs", "pm2.json",
		"tsconfig.json", "Dockerfile":
		return true
	}
	return strings.HasSuffix(p, "/package.json")
}

func (g *GitAPI) getJSON(ctx context.Context, rawURL, token string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("accept", "application/json")
	res, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("git api %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(res.Body).Decode(dest)
}

func (g *GitAPI) getRaw(ctx context.Context, rawURL, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("authorization", "Bearer "+token)
	res, err := g.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("git api %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return string(b), err
}
