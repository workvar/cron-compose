// Package connectors exposes the connector cache (service managers discovered on each
// server) over the REST API. Phase A is read-only: the agent reports what is installed
// via ConnectorEvent, the agent gateway upserts it, and these endpoints read it back.
// Write operations (validate/apply/lifecycle) arrive in later phases. See
// docs/connectors.md.
package connectors

import "time"

// Connector mirrors a row in the connectors table: one service manager on one server.
type Connector struct {
	ID           string            `json:"id"`
	ServerID     string            `json:"server_id"`
	ServerName   string            `json:"server_name,omitempty"` // populated by the overview list only
	Kind         string            `json:"kind"`
	Instance     string            `json:"instance"`
	Version      string            `json:"version,omitempty"`
	Status       string            `json:"status"`
	Manageable   bool              `json:"manageable"`
	Capabilities map[string]bool   `json:"capabilities"`
	ConfigPaths  []string          `json:"config_paths"`
	ObjectCount  int               `json:"object_count"`
	Detail       map[string]string `json:"detail,omitempty"`
	LastSeenAt   *time.Time        `json:"last_seen_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Resource is a config file or lifecycle object within a connector.
type Resource struct {
	ID          string            `json:"id"`
	ConnectorID string            `json:"connector_id"`
	Type        string            `json:"type"` // config_file|object
	Ref         string            `json:"ref"`
	Name        string            `json:"name"`
	State       string            `json:"state,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`
	SizeBytes   int64             `json:"size_bytes,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
