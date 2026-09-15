package deploys

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CommitStatus is one status to attach to a commit. The state names are GitHub's;
// PostCommitStatus translates them for GitLab, which uses a different vocabulary for
// the same idea.
type CommitStatus struct {
	State       string // pending | success | failure | error
	Context     string // the check's name, e.g. croncompose/deploy
	Description string
	TargetURL   string
}

// PostCommitStatus attaches a status to a commit. A failure here is reported to the
// caller but is never allowed to fail a deploy: the deploy already happened, and the
// status is a report about it.
func (g *GitAPI) PostCommitStatus(ctx context.Context, provider, token, fullName, repoID, sha string, st CommitStatus) error {
	if sha == "" {
		return fmt.Errorf("commit status needs a commit sha")
	}
	if provider == "gitlab" {
		return g.postGitLabStatus(ctx, token, fullName, repoID, sha, st)
	}
	return g.postGitHubStatus(ctx, token, fullName, sha, st)
}

func (g *GitAPI) postGitHubStatus(ctx context.Context, token, fullName, sha string, st CommitStatus) error {
	payload := map[string]string{
		"state":       st.State,
		"context":     st.Context,
		"description": st.Description,
		"target_url":  st.TargetURL,
	}
	endpoint := fmt.Sprintf("%s/repos/%s/statuses/%s", g.githubAPI(), fullName, sha)
	code, raw, err := g.doJSON(ctx, http.MethodPost, endpoint, token, payload, nil)
	if err != nil || code >= 300 {
		return fmt.Errorf("github status %d: %s", code, strings.TrimSpace(string(raw)))
	}
	return nil
}

// gitlabState maps GitHub's status vocabulary onto GitLab's. GitLab has no separate
// "error", so an infrastructure failure and a build failure both land on "failed".
func gitlabState(state string) string {
	switch state {
	case stateSuccess:
		return "success"
	case statePending:
		return "running"
	default:
		return "failed"
	}
}

func (g *GitAPI) postGitLabStatus(ctx context.Context, token, fullName, repoID, sha string, st CommitStatus) error {
	id := repoID
	if id == "" {
		id = url.PathEscape(fullName)
	}
	payload := map[string]string{
		"state":       gitlabState(st.State),
		"name":        st.Context,
		"description": st.Description,
		"target_url":  st.TargetURL,
	}
	endpoint := fmt.Sprintf("%s/projects/%s/statuses/%s", g.gitlabAPI(), id, sha)
	code, raw, err := g.doJSON(ctx, http.MethodPost, endpoint, token, payload, nil)
	// GitLab rejects a transition it considers invalid (running -> running, say) with
	// 400. That is not worth reporting as a failure: the status is already where we
	// want it.
	if code == http.StatusBadRequest {
		return nil
	}
	if err != nil || code >= 300 {
		return fmt.Errorf("gitlab status %d: %s", code, strings.TrimSpace(string(raw)))
	}
	return nil
}
