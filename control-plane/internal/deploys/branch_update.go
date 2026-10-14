package deploys

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// BranchUpdateStatus is GET /deploys/:id/update-check — whether the connected
// branch tip has moved past the last successful deploy of that branch.
type BranchUpdateStatus struct {
	Branch          string     `json:"branch"`
	DeployedSHA     string     `json:"deployed_sha,omitempty"`
	RemoteSHA       string     `json:"remote_sha,omitempty"`
	UpdateAvailable bool       `json:"update_available"`
	AheadBy         int        `json:"ahead_by,omitempty"`
	BehindBy        int        `json:"behind_by,omitempty"`
	DeployedAt      *time.Time `json:"deployed_at,omitempty"`
	Busy            bool       `json:"busy,omitempty"`
	Message         string     `json:"message,omitempty"`
}

// BranchTip returns the current tip commit SHA of branch for the given repo.
func (g *GitAPI) BranchTip(ctx context.Context, provider, token, fullName, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", fmt.Errorf("branch is required")
	}
	if provider == "gitlab" {
		return g.branchTipGitLab(ctx, token, fullName, branch)
	}
	return g.branchTipGitHub(ctx, token, fullName, branch)
}

func (g *GitAPI) branchTipGitHub(ctx context.Context, token, fullName, branch string) (string, error) {
	var raw struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	u := fmt.Sprintf("%s/repos/%s/branches/%s", g.githubAPI(), fullName, url.PathEscape(branch))
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return "", err
	}
	if raw.Commit.SHA == "" {
		return "", fmt.Errorf("github branch %s has no tip sha", branch)
	}
	return raw.Commit.SHA, nil
}

func (g *GitAPI) branchTipGitLab(ctx context.Context, token, fullName, branch string) (string, error) {
	enc := url.PathEscape(fullName)
	var raw struct {
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	u := fmt.Sprintf("%s/projects/%s/repository/branches/%s", g.gitlabAPI(), enc, url.PathEscape(branch))
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return "", err
	}
	if raw.Commit.ID == "" {
		return "", fmt.Errorf("gitlab branch %s has no tip sha", branch)
	}
	return raw.Commit.ID, nil
}

// CompareCommits reports how far head is ahead of / behind base for the same repo.
// On providers that do not expose a compare API cleanly, ahead/behind stay 0.
func (g *GitAPI) CompareCommits(ctx context.Context, provider, token, fullName, base, head string) (ahead, behind int, err error) {
	if base == "" || head == "" || sameCommit(base, head) {
		return 0, 0, nil
	}
	if provider == "gitlab" {
		return g.compareGitLab(ctx, token, fullName, base, head)
	}
	return g.compareGitHub(ctx, token, fullName, base, head)
}

func (g *GitAPI) compareGitHub(ctx context.Context, token, fullName, base, head string) (int, int, error) {
	var raw struct {
		AheadBy  int `json:"ahead_by"`
		BehindBy int `json:"behind_by"`
	}
	u := fmt.Sprintf("%s/repos/%s/compare/%s...%s",
		g.githubAPI(), fullName, url.PathEscape(base), url.PathEscape(head))
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return 0, 0, err
	}
	return raw.AheadBy, raw.BehindBy, nil
}

func (g *GitAPI) compareGitLab(ctx context.Context, token, fullName, base, head string) (int, int, error) {
	enc := url.PathEscape(fullName)
	var raw struct {
		Commits []struct {
			ID string `json:"id"`
		} `json:"commits"`
	}
	// from=base, to=head → commits reachable from head but not base (remote ahead).
	u := fmt.Sprintf("%s/projects/%s/repository/compare?from=%s&to=%s",
		g.gitlabAPI(), enc, url.QueryEscape(base), url.QueryEscape(head))
	if err := g.getJSON(ctx, u, token, &raw); err != nil {
		return 0, 0, err
	}
	return len(raw.Commits), 0, nil
}

func sameCommit(a, b string) bool {
	a = strings.TrimSpace(strings.ToLower(a))
	b = strings.TrimSpace(strings.ToLower(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	// Allow abbreviated SHAs when one side is a prefix of the other.
	if len(a) >= 7 && len(b) >= 7 && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a)) {
		return true
	}
	return false
}
