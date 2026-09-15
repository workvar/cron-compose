package githubapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TokenForRepo is the one call the rest of the codebase needs: hand it "owner/repo"
// and get a bearer token scoped to the installation covering that repo, or an error
// saying the App is not installed there. Both lookups are cached, so a busy project
// costs one HTTP round trip an hour.
func (a *App) TokenForRepo(ctx context.Context, fullName string) (string, error) {
	if a == nil {
		return "", ErrNotConfigured
	}
	id, err := a.installationForRepo(ctx, fullName)
	if err != nil {
		return "", err
	}
	return a.InstallationToken(ctx, id)
}

// ErrNotConfigured means no GitHub App is set up. Callers treat it as "fall back to
// whatever credentials you already had" rather than as a failure.
var ErrNotConfigured = fmt.Errorf("github app is not configured")

// ErrNotInstalled means the App exists but the owner has not installed it on this
// repo. It is a setup problem a person has to fix, so it is worth distinguishing.
var ErrNotInstalled = fmt.Errorf("github app is not installed on this repository")

// installationForRepo resolves which installation covers a repo. The mapping only
// changes when someone installs or uninstalls the App, so it is cached for the life
// of the process; a restart is the recovery path for a stale entry.
func (a *App) installationForRepo(ctx context.Context, fullName string) (int64, error) {
	a.mu.Lock()
	id, ok := a.installs[fullName]
	a.mu.Unlock()
	if ok {
		return id, nil
	}

	assertion, err := a.appJWT(time.Now())
	if err != nil {
		return 0, err
	}
	var out struct {
		ID int64 `json:"id"`
	}
	url := a.apiBase + "/repos/" + fullName + "/installation"
	if err := a.do(ctx, http.MethodGet, url, assertion, &out); err != nil {
		if strings.Contains(err.Error(), "404") {
			return 0, ErrNotInstalled
		}
		return 0, err
	}
	if out.ID == 0 {
		return 0, ErrNotInstalled
	}
	a.mu.Lock()
	a.installs[fullName] = out.ID
	a.mu.Unlock()
	return out.ID, nil
}

// do performs one App-authenticated request. The JWT (not an installation token) is
// the credential for the two endpoints in this package.
func (a *App) do(ctx context.Context, method, url, assertion string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "Bearer "+assertion)
	req.Header.Set("accept", "application/vnd.github+json")
	req.Header.Set("x-github-api-version", "2022-11-28")
	req.Header.Set("user-agent", "CronCompose")

	res, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("github app api %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest != nil && len(raw) > 0 {
		return json.Unmarshal(raw, dest)
	}
	return nil
}
