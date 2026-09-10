package deploys

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RepoFile is a path+content pair written into a git repo.
type RepoFile struct {
	Path    string
	Content string
}

// EnsureWebhook creates a push webhook on the provider repo. A duplicate hook is not an error.
func (g *GitAPI) EnsureWebhook(ctx context.Context, provider, token, fullName, repoID, hookURL, secret string) error {
	if provider == "gitlab" {
		return g.ensureGitLabHook(ctx, token, fullName, repoID, hookURL, secret)
	}
	return g.ensureGitHubHook(ctx, token, fullName, hookURL, secret)
}

func (g *GitAPI) ensureGitHubHook(ctx context.Context, token, fullName, hookURL, secret string) error {
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": []string{"push"},
		"config": map[string]string{
			"url":          hookURL,
			"content_type": "json",
			"secret":       secret,
			"insecure_ssl": "0",
		},
	}
	code, raw, err := g.doJSON(ctx, http.MethodPost, g.githubAPI()+"/repos/"+fullName+"/hooks", token, payload, nil)
	if err != nil && code != http.StatusUnprocessableEntity && code != http.StatusConflict {
		if code >= 300 {
			return fmt.Errorf("github webhook %d: %s", code, strings.TrimSpace(string(raw)))
		}
		return err
	}
	if code == http.StatusUnprocessableEntity || code == http.StatusConflict {
		return nil
	}
	return err
}

func (g *GitAPI) ensureGitLabHook(ctx context.Context, token, fullName, repoID, hookURL, secret string) error {
	id := repoID
	if id == "" {
		id = url.PathEscape(fullName)
	}
	payload := map[string]any{
		"url":                     hookURL,
		"token":                   secret,
		"push_events":             true,
		"enable_ssl_verification": true,
	}
	code, raw, err := g.doJSON(ctx, http.MethodPost, g.gitlabAPI()+"/projects/"+id+"/hooks", token, payload, nil)
	if err != nil && code != http.StatusBadRequest && code != http.StatusConflict {
		if code >= 300 {
			return fmt.Errorf("gitlab webhook %d: %s", code, strings.TrimSpace(string(raw)))
		}
		return err
	}
	return nil
}

// EnsureRepoFiles commits the given files onto branch (create or update).
func (g *GitAPI) EnsureRepoFiles(ctx context.Context, provider, token, fullName, repoID, branch string, files []RepoFile) error {
	if len(files) == 0 {
		return nil
	}
	if branch == "" {
		branch = "main"
	}
	if provider == "gitlab" {
		return g.commitGitLab(ctx, token, fullName, repoID, branch, files)
	}
	return g.commitGitHub(ctx, token, fullName, branch, files)
}

func (g *GitAPI) commitGitHub(ctx context.Context, token, fullName, branch string, files []RepoFile) error {
	for _, f := range files {
		var existing struct {
			SHA string `json:"sha"`
		}
		getURL := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", g.githubAPI(), fullName, f.Path, url.QueryEscape(branch))
		_, _, _ = g.doJSON(ctx, http.MethodGet, getURL, token, nil, &existing)
		payload := map[string]any{
			"message": "chore: add CronCompose deploy files",
			"content": base64.StdEncoding.EncodeToString([]byte(f.Content)),
			"branch":  branch,
		}
		if existing.SHA != "" {
			payload["sha"] = existing.SHA
			payload["message"] = "chore: update CronCompose deploy files"
		}
		putURL := fmt.Sprintf("%s/repos/%s/contents/%s", g.githubAPI(), fullName, f.Path)
		code, raw, err := g.doJSON(ctx, http.MethodPut, putURL, token, payload, nil)
		if err != nil || code >= 300 {
			return fmt.Errorf("github contents %s %d: %s", f.Path, code, strings.TrimSpace(string(raw)))
		}
	}
	return nil
}

func (g *GitAPI) commitGitLab(ctx context.Context, token, fullName, repoID, branch string, files []RepoFile) error {
	id := repoID
	if id == "" {
		id = url.PathEscape(fullName)
	}
	try := func(action string) (int, []byte, error) {
		actions := make([]map[string]string, 0, len(files))
		for _, f := range files {
			actions = append(actions, map[string]string{
				"action":    action,
				"file_path": f.Path,
				"content":   f.Content,
			})
		}
		payload := map[string]any{
			"branch":         branch,
			"commit_message": "chore: add CronCompose deploy files",
			"actions":        actions,
		}
		return g.doJSON(ctx, http.MethodPost, g.gitlabAPI()+"/projects/"+id+"/repository/commits", token, payload, nil)
	}
	code, raw, err := try("create")
	if code >= 300 {
		code, raw, err = try("update")
	}
	if err != nil || code >= 300 {
		return fmt.Errorf("gitlab commit %d: %s", code, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (g *GitAPI) doJSON(ctx context.Context, method, rawURL, token string, payload, dest any) (int, []byte, error) {
	var rdr io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("accept", "application/vnd.github+json, application/json")
	req.Header.Set("user-agent", "CronCompose")
	if payload != nil {
		req.Header.Set("content-type", "application/json")
	}
	res, err := g.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return res.StatusCode, raw, err
	}
	if res.StatusCode >= 300 {
		return res.StatusCode, raw, fmt.Errorf("git api %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, dest); err != nil {
			return res.StatusCode, raw, err
		}
	}
	return res.StatusCode, raw, nil
}
