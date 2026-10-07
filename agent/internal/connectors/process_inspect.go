package connectors

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"sort"
	"strings"
)

// ObjectInspector returns a full process detail (including env) for one object ref.
// Used by Deploy import after passkey step-up; default Resources stay redacted.
type ObjectInspector interface {
	Inspect(ctx context.Context, inst Instance, ref string) Result
}

// ProcessDetail is the JSON payload for an inspect op.
type ProcessDetail struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    string            `json:"args,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	State   string            `json:"state,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"`
	Git     *GitRemote        `json:"git,omitempty"`
}

// GitRemote is a best-effort origin detected from a process working directory.
type GitRemote struct {
	Remote       string `json:"remote,omitempty"`
	Provider     string `json:"provider,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
	Branch       string `json:"branch,omitempty"`
	CloneURL     string `json:"clone_url,omitempty"`
}

func inspectJSON(d ProcessDetail) Result {
	if d.Cwd != "" && d.Git == nil {
		if g := gitFromCwd(context.Background(), d.Cwd); g != nil {
			d.Git = g
		}
	}
	b, err := json.Marshal(d)
	if err != nil {
		return fail(StatusFailed, "marshal inspect: "+err.Error())
	}
	return Result{
		Status:  StatusSucceeded,
		Message: "inspected " + d.Name,
		Content: b,
		Payload: b,
	}
}

// gitFromCwd reads origin + branch from a checkout. Best-effort; returns nil on miss.
func gitFromCwd(ctx context.Context, cwd string) *GitRemote {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" || !has("git") {
		return nil
	}
	remote, err := run(ctx, "git", "-C", cwd, "remote", "get-url", "origin")
	if err != nil {
		return nil
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return nil
	}
	g := &GitRemote{Remote: remote, CloneURL: remote}
	if provider, full := parseGitRemote(remote); full != "" {
		g.Provider = provider
		g.RepoFullName = full
	}
	if out, err := run(ctx, "git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		br := strings.TrimSpace(out)
		if br != "" && br != "HEAD" {
			g.Branch = br
		}
	}
	return g
}

// parseGitRemote turns an origin URL into (provider, owner/repo).
func parseGitRemote(raw string) (provider, fullName string) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".git")
	if strings.HasPrefix(raw, "git@") {
		// git@github.com:owner/repo
		rest := strings.TrimPrefix(raw, "git@")
		host, repo, ok := strings.Cut(rest, ":")
		if !ok {
			return "", ""
		}
		return hostProvider(host), strings.TrimPrefix(repo, "/")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", ""
	}
	p := strings.Trim(path.Clean(u.Path), "/")
	if p == "" || !strings.Contains(p, "/") {
		return "", ""
	}
	return hostProvider(u.Hostname()), p
}

func hostProvider(host string) string {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "github"):
		return "github"
	case strings.Contains(host, "gitlab"):
		return "gitlab"
	default:
		return host
	}
}

// envKeysCSV returns a comma-separated sorted-ish list of env keys (values omitted).
func envKeysCSV(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		if k == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// parseEnvList turns KEY=VAL lines (systemd Environment= / docker Env) into a map.
func parseEnvList(items []string) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		k, v, ok := strings.Cut(item, "=")
		if !ok || k == "" {
			continue
		}
		out[k] = v
	}
	return out
}
