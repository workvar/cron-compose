package notify

// Browser push event names. Stored on each subscription as a jsonb array.
const (
	PushDeployStarted   = "deploy_started"
	PushDeploySucceeded = "deploy_succeeded"
	PushDeployFailed    = "deploy_failed"
	PushJobFailed       = "job_failed"
	PushUpdateAvailable = "update_available"
)

// DefaultPushEvents is what a new browser subscription opts into.
var DefaultPushEvents = []string{
	PushDeployStarted,
	PushDeploySucceeded,
	PushDeployFailed,
	PushJobFailed,
	PushUpdateAvailable,
}

// ValidPushEvent reports whether e is a known browser-push event.
func ValidPushEvent(e string) bool {
	switch e {
	case PushDeployStarted, PushDeploySucceeded, PushDeployFailed, PushJobFailed, PushUpdateAvailable:
		return true
	}
	return false
}

// PushPayload is the JSON body delivered to the service worker.
type PushPayload struct {
	Title   string            `json:"title"`
	Body    string            `json:"body"`
	URL     string            `json:"url,omitempty"`
	Tag     string            `json:"tag,omitempty"`
	Event   string            `json:"event"`
	Icon    string            `json:"icon,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
}
