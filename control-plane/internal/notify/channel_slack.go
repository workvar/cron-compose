package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// deliverSlack posts to a Slack incoming webhook.
//
// The message is built as Block Kit rather than a plain string so the important part
// (which job, on which server) is readable at a glance in a notification, with the
// error text below it. Slack truncates aggressively, so the error is capped well
// under the block limit.
func (n *Notifier) deliverSlack(ctx context.Context, t Target, ev RunFailedEvent) error {
	if t.URL == "" {
		return errors.New("slack target has no incoming-webhook url")
	}

	headline := fmt.Sprintf("%s %s on %s",
		statusEmoji(ev.Status), nameOr(ev.JobName, ev.JobID), nameOr(ev.ServerName, ev.ServerID))

	fields := []map[string]string{
		{"type": "mrkdwn", "text": "*Status*\n" + ev.Status},
		{"type": "mrkdwn", "text": fmt.Sprintf("*Exit code*\n%d", ev.ExitCode)},
		{"type": "mrkdwn", "text": "*Duration*\n" + humanMillis(ev.DurationMs)},
	}
	if ev.RunURL != "" {
		fields = append(fields, map[string]string{
			"type": "mrkdwn", "text": "*Run*\n<" + ev.RunURL + "|open>",
		})
	}

	blocks := []map[string]any{
		{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": "*" + headline + "*"}},
		{"type": "section", "fields": fields},
	}
	if ev.Error != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]string{"type": "mrkdwn", "text": "```" + truncate(ev.Error, 2500) + "```"},
		})
	}

	payload := map[string]any{"text": headline, "blocks": blocks}
	if ch := t.Config["channel"]; ch != "" {
		// Only workspaces using a legacy webhook honour this; newer ones post to the
		// channel the webhook was installed for and ignore it, which is harmless.
		payload["channel"] = ch
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	return n.doRequest(req)
}

func statusEmoji(status string) string {
	switch status {
	case "timed_out":
		return ":hourglass:"
	case "canceled":
		return ":black_square_for_stop:"
	case "skipped":
		return ":fast_forward:"
	}
	return ":x:"
}

func nameOr(name, id string) string {
	if name != "" {
		return name
	}
	return id
}

func humanMillis(ms int32) string {
	switch {
	case ms < 1000:
		return fmt.Sprintf("%dms", ms)
	case ms < 60_000:
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dm %ds", ms/60_000, (ms%60_000)/1000)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
