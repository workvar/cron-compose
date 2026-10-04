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
		Language      string `json:"language"`
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
			Language: r.Language,
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

// ListBranches lists a repo's branches, most-recently-updated first where the
// provider's API supports that ordering. Used by the import wizard's searchable
// branch picker so it is never limited to typing a name from memory.
func (g *GitAPI) ListBranches(ctx context.Context, provider, token, fullName string) ([]Branch, error) {
	if provider == "gitlab" {
		return g.listBranchesGitLab(ctx, token, fullName)
	}
	return g.listBranchesGitHub(ctx, token, fullName)
}

func (g *GitAPI) listBranchesGitHub(ctx context.Context, token, fullName string) ([]Branch, error) {
	var meta struct {
		DefaultBranch string `json:"default_branch"`
	}
	_ = g.getJSON(ctx, g.githubAPI()+"/repos/"+fullName, token, &meta)
	var raw []struct {
		Name string `json:"name"`
	}
	u := fmt.Sprintf("%s/repos/%s/branches?per_page=100", g.githubAPI(), fullName)
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return nil, err
	}
	out := make([]Branch, 0, len(raw))
	for _, b := range raw {
		out = append(out, Branch{Name: b.Name, Default: b.Name == meta.DefaultBranch})
	}
	return out, nil
}

func (g *GitAPI) listBranchesGitLab(ctx context.Context, token, fullName string) ([]Branch, error) {
	enc := url.PathEscape(fullName)
	var meta struct {
		DefaultBranch string `json:"default_branch"`
	}
	_ = g.getJSON(ctx, g.gitlabAPI()+"/projects/"+enc, token, &meta)
	var raw []struct {
		Name string `json:"name"`
	}
	u := fmt.Sprintf("%s/projects/%s/repository/branches?per_page=100", g.gitlabAPI(), enc)
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return nil, err
	}
	out := make([]Branch, 0, len(raw))
	for _, b := range raw {
		out = append(out, Branch{Name: b.Name, Default: b.Name == meta.DefaultBranch})
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

// FetchFile reads one file from the repo at branch.
func (g *GitAPI) FetchFile(ctx context.Context, provider, token, fullName, branch, path string) (string, error) {
	if provider == "gitlab" {
		u := fmt.Sprintf("%s/projects/%s/repository/files/%s/raw?ref=%s", g.gitlabAPI(), url.PathEscape(fullName), url.PathEscape(path), url.QueryEscape(branch))
		return g.getRaw(ctx, u, token)
	}
	return g.githubFile(ctx, token, fullName, path, branch)
}

func (g *GitAPI) githubFile(ctx context.Context, token, fullName, path, branch string) (string, error) {
	if token == "" && g.githubBase == "" {
		// Anonymous reads go to raw.githubusercontent.com: the contents API allows
		// only 60 unauthenticated calls an hour, and inspect makes dozens.
		return g.getRaw(ctx, fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", fullName, url.PathEscape(branch), path), "")
	}
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

// ListSpecFiles lists croncompose.yml paths in a repo (recursive tree scan, capped).
func (g *GitAPI) ListSpecFiles(ctx context.Context, provider, token, fullName, branch string) ([]SpecFileEntry, error) {
	if provider == "gitlab" {
		return g.listSpecFilesGitLab(ctx, token, fullName, branch)
	}
	return g.listSpecFilesGitHub(ctx, token, fullName, branch)
}

func (g *GitAPI) listSpecFilesGitHub(ctx context.Context, token, fullName, branch string) ([]SpecFileEntry, error) {
	if branch == "" {
		var meta struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := g.getJSON(ctx, g.githubAPI()+"/repos/"+fullName, token, &meta); err != nil {
			return nil, err
		}
		branch = meta.DefaultBranch
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	u := fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", g.githubAPI(), fullName, url.PathEscape(branch))
	if err := g.getJSON(ctx, u, token, &tree); err != nil {
		return nil, err
	}
	var out []SpecFileEntry
	for _, t := range tree.Tree {
		if t.Type != "blob" || !IsSpecFileName(t.Path) {
			continue
		}
		out = append(out, SpecFileEntry{Path: t.Path})
		if len(out) >= MaxSpecFiles {
			break
		}
	}
	return out, nil
}

func (g *GitAPI) listSpecFilesGitLab(ctx context.Context, token, fullName, branch string) ([]SpecFileEntry, error) {
	enc := url.PathEscape(fullName)
	if branch == "" {
		var meta struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := g.getJSON(ctx, g.gitlabAPI()+"/projects/"+enc, token, &meta); err != nil {
			return nil, err
		}
		branch = meta.DefaultBranch
	}
	var out []SpecFileEntry
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("ref", branch)
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("recursive", "true")
		var pageItems []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		u := fmt.Sprintf("%s/projects/%s/repository/tree?%s", g.gitlabAPI(), enc, q.Encode())
		if err := g.getJSON(ctx, u, token, &pageItems); err != nil {
			return nil, err
		}
		if len(pageItems) == 0 {
			break
		}
		for _, t := range pageItems {
			if t.Type != "blob" || !IsSpecFileName(t.Path) {
				continue
			}
			out = append(out, SpecFileEntry{Path: t.Path})
			if len(out) >= MaxSpecFiles {
				return out, nil
			}
		}
		if len(pageItems) < 100 {
			break
		}
	}
	return out, nil
}

// ListDirs lists directories in a repo at path (shallow or recursive, capped).
func (g *GitAPI) ListDirs(ctx context.Context, provider, token, fullName, branch, path string, recursive bool) (DirList, error) {
	path = NormalizeRepoPath(path)
	if provider == "gitlab" {
		return g.listDirsGitLab(ctx, token, fullName, branch, path, recursive)
	}
	return g.listDirsGitHub(ctx, token, fullName, branch, path, recursive)
}

func (g *GitAPI) listDirsGitHub(ctx context.Context, token, fullName, branch, path string, recursive bool) (DirList, error) {
	out := DirList{Path: path, Recursive: recursive}
	if branch == "" {
		var meta struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := g.getJSON(ctx, g.githubAPI()+"/repos/"+fullName, token, &meta); err != nil {
			return out, err
		}
		branch = meta.DefaultBranch
	}
	if recursive {
		var tree struct {
			Tree []struct {
				Path string `json:"path"`
				Type string `json:"type"`
			} `json:"tree"`
		}
		u := fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", g.githubAPI(), fullName, url.PathEscape(branch))
		if err := g.getJSON(ctx, u, token, &tree); err != nil {
			return out, err
		}
		var paths []string
		for _, t := range tree.Tree {
			if t.Type == "tree" {
				paths = append(paths, t.Path)
			}
		}
		items, truncated := FilterAndCapDirs(paths, path, true, MaxDirEntries)
		out.Items = items
		out.Truncated = truncated
		return out, nil
	}
	var entries []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	u := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", g.githubAPI(), fullName, path, url.QueryEscape(branch))
	if path == "" {
		u = fmt.Sprintf("%s/repos/%s/contents/?ref=%s", g.githubAPI(), fullName, url.QueryEscape(branch))
	}
	if err := g.getJSON(ctx, u, token, &entries); err != nil {
		return out, err
	}
	var paths []string
	for _, e := range entries {
		if e.Type == "dir" {
			paths = append(paths, e.Path)
		}
	}
	items, truncated := FilterAndCapDirs(paths, path, false, MaxDirEntries)
	out.Items = items
	out.Truncated = truncated
	return out, nil
}

func (g *GitAPI) listDirsGitLab(ctx context.Context, token, fullName, branch, path string, recursive bool) (DirList, error) {
	out := DirList{Path: path, Recursive: recursive}
	enc := url.PathEscape(fullName)
	if branch == "" {
		var meta struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := g.getJSON(ctx, g.gitlabAPI()+"/projects/"+enc, token, &meta); err != nil {
			return out, err
		}
		branch = meta.DefaultBranch
	}
	base := fmt.Sprintf("%s/projects/%s/repository/tree", g.gitlabAPI(), enc)
	var allPaths []string
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("ref", branch)
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprintf("%d", page))
		if path != "" {
			q.Set("path", path)
		}
		if recursive {
			q.Set("recursive", "true")
		}
		var pageItems []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Type string `json:"type"`
		}
		u := base + "?" + q.Encode()
		if err := g.getJSON(ctx, u, token, &pageItems); err != nil {
			return out, err
		}
		for _, e := range pageItems {
			if e.Type == "tree" {
				allPaths = append(allPaths, e.Path)
			}
		}
		if len(pageItems) < 100 {
			break
		}
		if recursive && len(allPaths) >= MaxDirEntries {
			break
		}
	}
	items, truncated := FilterAndCapDirs(allPaths, path, recursive, MaxDirEntries)
	out.Items = items
	out.Truncated = truncated
	return out, nil
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
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
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
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
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
