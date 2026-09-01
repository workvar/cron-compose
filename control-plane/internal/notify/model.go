// Package notify fires notifications when a run finishes with a non-success status.
// Every channel is best-effort: a delivery failure is recorded on the target and
// logged, and never blocks or fails the run it is reporting on.
package notify

import "time"

// Channel kinds. Each maps to one channel_*.go file in this package.
const (
	KindWebhook = "webhook"
	KindSlack   = "slack"
	KindEmail   = "email"
)

// ValidKind reports whether a kind is one this build can deliver to. Rejecting at the
// API boundary is better than accepting a target that silently never fires.
func ValidKind(k string) bool {
	return k == KindWebhook || k == KindSlack || k == KindEmail
}

// Target is one notification destination.
type Target struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// URL is the webhook or Slack incoming-webhook endpoint. Unused by email.
	URL     string `json:"url,omitempty"`
	Enabled bool   `json:"enabled"`
	// Config is channel-specific. Email reads smtp_host, smtp_port, smtp_user,
	// smtp_password, from, to. Slack reads an optional channel override. Secrets in
	// here never leave the server; see Redacted.
	Config map[string]string `json:"config,omitempty"`
	// ServerLabels scopes this target to servers whose labels contain all of these.
	// Empty means the whole fleet.
	ServerLabels map[string]string `json:"server_labels,omitempty"`
	// OnStatuses limits which run outcomes fire this target. Empty means every
	// non-success outcome.
	OnStatuses  []string   `json:"on_statuses,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	LastFiredAt *time.Time `json:"last_fired_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// secretConfigKeys never leave the server. A password that round-trips through a form
// is a password in a browser history.
var secretConfigKeys = map[string]bool{
	"smtp_password": true,
	"token":         true,
	"auth_header":   true,
}

// Redacted returns a copy safe to hand to the API. Secret values become a fixed
// placeholder so the UI can show that something is set without showing what.
func (t Target) Redacted() Target {
	if len(t.Config) == 0 {
		return t
	}
	cfg := make(map[string]string, len(t.Config))
	for k, v := range t.Config {
		if secretConfigKeys[k] && v != "" {
			cfg[k] = redactedPlaceholder
			continue
		}
		cfg[k] = v
	}
	t.Config = cfg
	return t
}

// redactedPlaceholder is also what the API accepts back on a patch to mean "leave this
// secret alone", so an edit form can round-trip without the browser ever holding it.
const redactedPlaceholder = "********"

// Matches reports whether this target wants to hear about an event.
func (t Target) Matches(ev RunFailedEvent) bool {
	if len(t.OnStatuses) > 0 {
		found := false
		for _, s := range t.OnStatuses {
			if s == ev.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for k, v := range t.ServerLabels {
		if ev.ServerLabels[k] != v {
			return false
		}
	}
	return true
}

// CreateInput is the body of POST /notification-targets.
type CreateInput struct {
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	URL          string            `json:"url"`
	Config       map[string]string `json:"config"`
	ServerLabels map[string]string `json:"server_labels"`
	OnStatuses   []string          `json:"on_statuses"`
}

// PatchInput is the body of PATCH /notification-targets/:id. Every field is a pointer
// so "absent" is distinguishable from "set to empty".
type PatchInput struct {
	Name         *string            `json:"name"`
	URL          *string            `json:"url"`
	Enabled      *bool              `json:"enabled"`
	Config       *map[string]string `json:"config"`
	ServerLabels *map[string]string `json:"server_labels"`
	OnStatuses   *[]string          `json:"on_statuses"`
}

// RunFailedEvent describes the run that triggered a notification. Names and labels are
// resolved before dispatch: a Slack message saying "job 01HX... failed on server
// 01HY..." is not worth waking up for.
type RunFailedEvent struct {
	RunID        string            `json:"run_id"`
	JobID        string            `json:"job_id"`
	JobName      string            `json:"job_name,omitempty"`
	ServerID     string            `json:"server_id"`
	ServerName   string            `json:"server_name,omitempty"`
	ServerLabels map[string]string `json:"server_labels,omitempty"`
	Status       string            `json:"status"`
	ExitCode     int32             `json:"exit_code"`
	DurationMs   int32             `json:"duration_ms"`
	Error        string            `json:"error,omitempty"`
	// RunURL deep-links to the run in the UI. Empty when no public URL is configured.
	RunURL string `json:"run_url,omitempty"`
}
