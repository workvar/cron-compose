package deploy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Health checks are opt-in per project. Without one, a deploy is "successful" the
// moment the install script exits 0, which says nothing about whether the app came
// back up: the classic bad deploy installs cleanly and then crashes on its first
// request. With one configured, a failed probe fails the run, which is what lets
// auto-rollback catch that case.
const (
	healthInterval      = 3 * time.Second
	healthDefaultBudget = 60 * time.Second
	healthRequestTO     = 5 * time.Second
)

// healthConfig is the resolved probe for one app.
type healthConfig struct {
	URL    string
	Budget time.Duration
}

// resolveHealth turns the project's check plus the app's port into something to poll,
// or returns ok=false when no probe is configured. The probe always targets loopback:
// the agent runs on the same host as the app, and a deploy should not depend on the
// app being reachable from outside.
func resolveHealth(hc *agentv1.HealthCheck, appPort int32) (healthConfig, bool) {
	if hc == nil || hc.GetPath() == "" {
		return healthConfig{}, false
	}
	port := hc.GetPort()
	if port == 0 {
		port = appPort
	}
	if port <= 0 {
		return healthConfig{}, false
	}
	path := hc.GetPath()
	if path[0] != '/' {
		path = "/" + path
	}
	budget := time.Duration(hc.GetTimeoutSeconds()) * time.Second
	if budget <= 0 {
		budget = healthDefaultBudget
	}
	return healthConfig{
		URL:    "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))) + path,
		Budget: budget,
	}, true
}

// waitHealthy polls until the app answers with a non-error status or the budget runs
// out. Progress is logged so an operator watching the run sees what it is waiting on
// rather than a silent gap. The last failure reason is what the run reports.
func (m *Manager) waitHealthy(ctx context.Context, runID, token string, cfg healthConfig) error {
	deadline := time.Now().Add(cfg.Budget)
	client := &http.Client{
		Timeout: healthRequestTO,
		// A redirect from a health endpoint still means the app is answering.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	m.phaseLine(runID, token, phaseHealth, fmt.Sprintf("waiting for %s (up to %s)", cfg.URL, cfg.Budget))

	attempt := 0
	var lastErr string
	for {
		attempt++
		status, err := probe(ctx, client, cfg.URL)
		switch {
		case err != nil:
			lastErr = err.Error()
		case status >= 200 && status < 400:
			m.phaseLine(runID, token, phaseHealth, fmt.Sprintf("healthy: %d after %d attempt(s)", status, attempt))
			return nil
		default:
			lastErr = fmt.Sprintf("status %d", status)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check failed: %s did not become healthy within %s (last: %s)", cfg.URL, cfg.Budget, lastErr)
		}
		if attempt%5 == 0 {
			m.phaseLine(runID, token, phaseHealth, "still waiting: "+lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthInterval):
		}
	}
}

func probe(ctx context.Context, client *http.Client, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("user-agent", "CronCompose-healthcheck")
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, nil
}
