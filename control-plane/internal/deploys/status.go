package deploys

import (
	"context"
	"errors"
	"strings"

	"github.com/croncompose/croncompose/control-plane/internal/githubapp"
)

// Commit statuses put the deploy's real outcome next to the commit that caused it, so
// the question "is this commit live?" is answered where people already look instead of
// in a dashboard they have to remember to open.
//
// "Real outcome" is the point. A status that goes green when the install script exits
// 0 is worse than none at all, because it says the commit is serving when it may have
// crashed on boot. These statuses are posted after the health check, and a health
// failure is reported as a failure with a description that says the build was fine and
// the app did not come up.
const statusContext = "croncompose/deploy"

// GitHub status states. GitLab's are different strings, handled in the git client.
const (
	statePending = "pending"
	stateSuccess = "success"
	stateFailure = "failure"
	stateError   = "error"
)

// reportStatusStart marks the commit as deploying. It only fires when the commit is
// already known: a push or CI trigger reports it, and a rollback pins it. A manual
// redeploy learns the commit only once the agent has cloned, by which point a pending
// status would be a formality, so those get the final status alone.
func (h *handler) reportStatusStart(ctx context.Context, p Project, run Run) {
	if run.CommitSha == "" {
		return
	}
	h.postStatus(ctx, p, run, statePending, "Deploying to "+serverLabel(p), "")
}

// reportStatusFinish posts the outcome. phase is the stage that failed, empty on
// success.
func (h *handler) reportStatusFinish(ctx context.Context, p Project, run Run, status, phase string) {
	if run.CommitSha == "" {
		return
	}
	state, description := statusFor(p, run, status, phase)
	h.postStatus(ctx, p, run, state, description, phase)
}

// statusFor turns a run outcome into the state and one-line description a person sees
// on the commit. The descriptions are written to be read on their own, because in
// GitHub's UI that single line is often all anyone reads.
func statusFor(p Project, run Run, status, phase string) (state, description string) {
	if status == "succeeded" {
		if run.Trigger == triggerRollback {
			return stateSuccess, "Rolled back: " + serverLabel(p) + " is serving the last good commit"
		}
		if p.HealthPath != "" {
			return stateSuccess, "Deployed to " + serverLabel(p) + " and passed its health check"
		}
		return stateSuccess, "Deployed to " + serverLabel(p) + " (no health check configured)"
	}

	switch phase {
	case "health":
		// The distinction worth surfacing: the code is fine, the running app is not.
		return stateFailure, "Built and installed, but the app did not pass its health check"
	case "preflight":
		return stateError, "Server was not ready to deploy: " + firstLine(run.Error)
	case "clone":
		return stateError, "Could not check out this commit on " + serverLabel(p)
	case "install":
		return stateFailure, "Install or build failed on " + serverLabel(p)
	case "release", "start":
		return stateFailure, "Built fine, but the app could not be started"
	default:
		return stateFailure, "Deploy failed: " + firstLine(run.Error)
	}
}

// postStatus sends one status, preferring the GitHub App's installation token over the
// importing user's OAuth token. The App is optional: without one, statuses still work
// and simply appear as the user who imported the repo.
func (h *handler) postStatus(ctx context.Context, p Project, run Run, state, description, phase string) {
	token, err := h.statusToken(ctx, p)
	if err != nil {
		// Never fail or delay a deploy over a status. An operator who cares will see
		// this in the control-plane log.
		h.log.Info("deploy: no credentials to post a commit status",
			"project_id", p.ID, "repo", p.RepoFullName, "err", err)
		return
	}
	target := strings.TrimRight(h.public, "/") + "/app/deploys/runs/" + run.ID
	err = h.git.PostCommitStatus(ctx, p.Provider, token, p.RepoFullName, p.RepoID, run.CommitSha, CommitStatus{
		State:       state,
		Context:     statusContext,
		Description: truncate(description, 140), // GitHub silently rejects longer
		TargetURL:   target,
	})
	if err != nil {
		h.log.Warn("deploy: commit status failed",
			"project_id", p.ID, "commit", shortSHA(run.CommitSha), "state", state, "phase", phase, "err", err)
		return
	}
	h.log.Info("deploy: commit status posted",
		"project_id", p.ID, "commit", shortSHA(run.CommitSha), "state", state)
}

// statusToken picks the credential for the status API: the App installation token when
// an App is set up and installed on the repo, otherwise the OAuth token of whoever
// imported the project.
func (h *handler) statusToken(ctx context.Context, p Project) (string, error) {
	if p.Provider == "github" && h.app.Configured() {
		token, err := h.app.TokenForRepo(ctx, p.RepoFullName)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, githubapp.ErrNotInstalled) {
			return "", err
		}
		// Installed nowhere yet: fall through to the user token so statuses keep
		// working during a migration to the App.
		h.log.Info("deploy: github app not installed on repo, using the importing user's token",
			"repo", p.RepoFullName)
	}
	if p.CreatedBy == nil {
		return "", errors.New("project has no owner to borrow a git token from")
	}
	token, err := h.conns.Token(ctx, *p.CreatedBy, p.Provider)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("git grant missing")
	}
	return token, nil
}

func serverLabel(p Project) string {
	if len(p.ServerID) > 8 {
		return "server " + p.ServerID[:8]
	}
	return "the server"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "no detail reported"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
