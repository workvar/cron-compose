package notify

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/metrics"
)

// Notifier loads the targets that care about an event and delivers to each.
//
// Everything here runs off the run-completion path in a goroutine. A slow SMTP server
// must never slow down the agent stream, so the only coupling back is a recorded
// delivery status on the target row.
type Notifier struct {
	store   *Store
	pool    *pgxpool.Pool
	log     *slog.Logger
	client  *http.Client
	baseURL string
}

// NewNotifier wires a Notifier. baseURL is the public URL of the UI, used to build
// deep links into notifications; empty simply omits the links.
func NewNotifier(store *Store, pool *pgxpool.Pool, log *slog.Logger, baseURL string) *Notifier {
	return &Notifier{
		store:   store,
		pool:    pool,
		log:     log,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// FireRunFailed matches agentgw.FailedRunHook.
func (n *Notifier) FireRunFailed(serverID, jobID, runID, status string, exitCode, durationMs int32, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ev := RunFailedEvent{
		RunID:      runID,
		JobID:      jobID,
		ServerID:   serverID,
		Status:     status,
		ExitCode:   exitCode,
		DurationMs: durationMs,
		Error:      errMsg,
	}
	n.enrich(ctx, &ev)

	targets, err := n.store.EnabledList(ctx)
	if err != nil {
		n.log.Warn("notify: list targets failed", "err", err)
		return
	}

	// Deliveries run inside this function's context, so the deadline above bounds the
	// whole fan-out. Waiting for them keeps that context alive until they finish.
	done := make(chan struct{})
	pending := 0
	for _, t := range targets {
		if !t.Matches(ev) {
			continue
		}
		pending++
		go func(target Target) {
			defer func() { done <- struct{}{} }()
			n.deliverAndRecord(ctx, target, ev)
		}(t)
	}
	for i := 0; i < pending; i++ {
		<-done
	}
}

// Test delivers a synthetic event to one target and returns the delivery error, so the
// operator finds out their SMTP password is wrong when they save it rather than at 3am
// when a job actually fails.
func (n *Notifier) Test(ctx context.Context, t Target) error {
	ev := RunFailedEvent{
		RunID:      "test",
		JobID:      "test",
		JobName:    "Test notification",
		ServerID:   "test",
		ServerName: "CronCompose",
		Status:     "failed",
		ExitCode:   1,
		DurationMs: 1234,
		Error:      "This is a test notification from CronCompose. Nothing is broken.",
	}
	if n.baseURL != "" {
		ev.RunURL = n.baseURL + "/app"
	}
	err := n.deliver(ctx, t, ev)
	n.store.RecordDelivery(ctx, t.ID, errString(err))
	return err
}

func (n *Notifier) deliverAndRecord(ctx context.Context, t Target, ev RunFailedEvent) {
	err := n.deliver(ctx, t, ev)
	outcome := "delivered"
	if err != nil {
		outcome = "failed"
	}
	metrics.NotificationsTotal.WithLabelValues(t.Kind, outcome).Inc()
	if err != nil {
		n.log.Warn("notify: delivery failed", "target", t.Name, "kind", t.Kind, "err", err)
	}
	n.store.RecordDelivery(ctx, t.ID, errString(err))
}

// deliver dispatches to the channel implementation for this target's kind.
func (n *Notifier) deliver(ctx context.Context, t Target, ev RunFailedEvent) error {
	switch t.Kind {
	case KindSlack:
		return n.deliverSlack(ctx, t, ev)
	case KindEmail:
		return n.deliverEmail(ctx, t, ev)
	case KindWebhook:
		return n.deliverWebhook(ctx, t, ev)
	}
	return errors.New("unknown notification kind: " + t.Kind)
}

// enrich fills in the human-readable names and the server's labels. The labels are not
// decoration: label scoping on a target is matched against them.
//
// A lookup failure is not fatal. A notification carrying ids is worse than one carrying
// names, and far better than no notification.
func (n *Notifier) enrich(ctx context.Context, ev *RunFailedEvent) {
	if n.baseURL != "" && ev.RunID != "" {
		ev.RunURL = n.baseURL + "/app/runs/" + ev.RunID
	}
	ev.ServerLabels = map[string]string{}

	var labels []byte
	if err := n.pool.QueryRow(ctx,
		`select coalesce(name,''), coalesce(labels,'{}'::jsonb) from servers where id = $1`,
		ev.ServerID).Scan(&ev.ServerName, &labels); err != nil {
		n.log.Debug("notify: server lookup failed", "err", err, "server_id", ev.ServerID)
	} else {
		_ = json.Unmarshal(labels, &ev.ServerLabels)
	}

	if err := n.pool.QueryRow(ctx,
		`select coalesce(name,'') from jobs where id = $1`, ev.JobID).Scan(&ev.JobName); err != nil {
		n.log.Debug("notify: job lookup failed", "err", err, "job_id", ev.JobID)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
