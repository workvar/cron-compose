package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// sendPush delivers payload to every subscription that opted into event.
// Gone / expired endpoints (HTTP 404 / 410) are deleted automatically.
func (n *Notifier) sendPush(ctx context.Context, event string, payload PushPayload) {
	if n == nil || n.pool == nil {
		return
	}
	store := newPushStore(n.pool)
	subs, err := store.listForEvent(ctx, event)
	if err != nil {
		n.log.Warn("push: list subscriptions failed", "err", err, "event", event)
		return
	}
	if len(subs) == 0 {
		return
	}
	vapid, err := EnsureVAPID(ctx, n.pool)
	if err != nil {
		n.log.Warn("push: vapid unavailable", "err", err)
		return
	}
	payload.Event = event
	if payload.Icon == "" && n.baseURL != "" {
		payload.Icon = n.baseURL + "/app/logo.png"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	subscriber := vapidSubscriber(vapid.Subject)
	done := make(chan struct{})
	pending := 0
	for _, sub := range subs {
		pending++
		go func(s PushSubscription) {
			defer func() { done <- struct{}{} }()
			n.deliverPush(ctx, vapid, subscriber, s, body, store)
		}(sub)
	}
	for i := 0; i < pending; i++ {
		select {
		case <-done:
		case <-ctx.Done():
			return
		}
	}
}

func (n *Notifier) deliverPush(ctx context.Context, vapid VAPID, subscriber string, s PushSubscription, body []byte, store *pushStore) {
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: s.Endpoint,
		Keys: webpush.Keys{
			Auth:   s.Auth,
			P256dh: s.P256dh,
		},
	}, &webpush.Options{
		Subscriber:      subscriber,
		VAPIDPublicKey:  vapid.PublicKey,
		VAPIDPrivateKey: vapid.PrivateKey,
		TTL:             60 * 60 * 12,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		n.log.Warn("push: send failed", "err", err, "sub", s.ID)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		_ = store.deleteByEndpoint(ctx, s.Endpoint)
		n.log.Info("push: dropped expired subscription", "sub", s.ID)
		return
	}
	if resp.StatusCode >= 300 {
		n.log.Warn("push: unexpected status", "status", resp.StatusCode, "sub", s.ID)
	}
}

func vapidSubscriber(subject string) string {
	s := strings.TrimSpace(subject)
	s = strings.TrimPrefix(s, "mailto:")
	if s == "" {
		return "croncompose@localhost"
	}
	return s
}

// FirePushDeployStarted notifies browsers a deploy run began.
func (n *Notifier) FirePushDeployStarted(serverID, projectID, runID, branch, trigger string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ev := RunFailedEvent{
		EventKind: EventDeploy,
		RunID:     runID,
		ProjectID: projectID,
		ServerID:  serverID,
		Branch:    branch,
		Trigger:   trigger,
		Status:    "running",
	}
	n.enrichDeploy(ctx, &ev)
	title := "Deploy started"
	if ev.ProjectName != "" {
		title = ev.ProjectName + " deploying"
	}
	body := ev.ServerName
	if branch != "" {
		body = fmt.Sprintf("%s · %s", body, branch)
	}
	if trigger != "" && trigger != "manual" {
		body = fmt.Sprintf("%s · %s", body, trigger)
	}
	url := ev.RunURL
	n.sendPush(ctx, PushDeployStarted, PushPayload{
		Title: title,
		Body:  body,
		URL:   url,
		Tag:   "deploy-" + runID,
		Data:  map[string]string{"run_id": runID, "project_id": projectID},
	})
}

// FirePushDeployFinished notifies browsers of deploy success or failure.
func (n *Notifier) FirePushDeployFinished(serverID, projectID, runID, status, branch, trigger, phase string, exitCode int32, errMsg string, rolledBack bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ev := RunFailedEvent{
		EventKind:  EventDeploy,
		RunID:      runID,
		ProjectID:  projectID,
		ServerID:   serverID,
		Branch:     branch,
		Trigger:    trigger,
		Phase:      phase,
		Status:     status,
		ExitCode:   exitCode,
		Error:      errMsg,
		RolledBack: rolledBack,
	}
	n.enrichDeploy(ctx, &ev)

	event := PushDeploySucceeded
	title := "Deploy succeeded"
	if status != "succeeded" {
		event = PushDeployFailed
		title = "Deploy failed"
	}
	if rolledBack && status == "succeeded" {
		title = "Deploy rolled back"
	}
	if ev.ProjectName != "" {
		if event == PushDeploySucceeded {
			title = ev.ProjectName + " is live"
		} else {
			title = ev.ProjectName + " deploy failed"
		}
	}
	body := pushDeployBody(ev)
	n.sendPush(ctx, event, PushPayload{
		Title: title,
		Body:  body,
		URL:   ev.RunURL,
		Tag:   "deploy-" + runID,
		Data:  map[string]string{"run_id": runID, "status": status, "project_id": projectID},
	})
}

// FirePushJobFailed notifies browsers a scheduled job run failed.
func (n *Notifier) FirePushJobFailed(serverID, jobID, runID, status string, exitCode, durationMs int32, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ev := RunFailedEvent{
		EventKind:  EventJobRun,
		RunID:      runID,
		JobID:      jobID,
		ServerID:   serverID,
		Status:     status,
		ExitCode:   exitCode,
		DurationMs: durationMs,
		Error:      errMsg,
	}
	n.enrich(ctx, &ev)
	title := "Job failed"
	if ev.JobName != "" {
		title = ev.JobName + " failed"
	}
	body := ev.ServerName
	if errMsg != "" {
		body = fmt.Sprintf("%s · %s", body, trimErr(errMsg, 120))
	}
	n.sendPush(ctx, PushJobFailed, PushPayload{
		Title: title,
		Body:  body,
		URL:   ev.RunURL,
		Tag:   "job-" + runID,
		Data:  map[string]string{"run_id": runID, "job_id": jobID},
	})
}

// FirePushUpdateAvailable notifies browsers a new agent/stack release is out.
func (n *Notifier) FirePushUpdateAvailable(version, releaseURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := n.baseURL + "/app/settings"
	if releaseURL != "" {
		url = releaseURL
	}
	n.sendPush(ctx, PushUpdateAvailable, PushPayload{
		Title: "Update available",
		Body:  "CronCompose " + version + " is ready to install on your servers.",
		URL:   url,
		Tag:   "update-" + version,
		Data:  map[string]string{"version": version},
	})
}

func trimErr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func pushDeployBody(ev RunFailedEvent) string {
	parts := []string{}
	if ev.ServerName != "" {
		parts = append(parts, ev.ServerName)
	}
	if ev.Branch != "" {
		parts = append(parts, ev.Branch)
	}
	if label := phaseLabel(ev.Phase); label != "" && ev.Status != "succeeded" {
		parts = append(parts, label)
	}
	if ev.Error != "" {
		parts = append(parts, trimErr(ev.Error, 100))
	} else if st := stateLabel(ev.ProjectState); st != "" {
		parts = append(parts, st)
	}
	if suf := rollbackSuffix(ev); suf != "" {
		parts = append(parts, strings.TrimSpace(suf))
	}
	return strings.Join(parts, " · ")
}
