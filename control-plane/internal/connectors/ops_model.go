package connectors

import "time"

// Operation is one connector command and its outcome: the connector equivalent of a
// job run. Rows are append-only, so the audit trail survives the discovery cache being
// rewritten on the next sweep.
type Operation struct {
	ID          string     `json:"id"`
	ConnectorID string     `json:"connector_id"`
	ServerID    string     `json:"server_id"`
	RequestID   string     `json:"request_id"`
	Op          string     `json:"op"`
	Action      string     `json:"action,omitempty"`
	Ref         string     `json:"ref,omitempty"`
	DryRun      bool       `json:"dry_run"`
	Status      string     `json:"status"`
	Message     string     `json:"message,omitempty"`
	Steps       []Step     `json:"steps"`
	ActorUserID *string    `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Step mirrors the agent's per-stage report (backup, validate, write, activate,
// health, rollback). Kept as a struct rather than raw JSON so the UI has a contract.
type Step struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Output   string `json:"output,omitempty"`
	ExitCode int32  `json:"exit_code,omitempty"`
}

// Snapshot is the content of a config file as it was before an apply overwrote it.
// This is what a rollback reads, so it stores the bytes rather than a diff.
type Snapshot struct {
	ID          string    `json:"id"`
	ConnectorID string    `json:"connector_id"`
	Ref         string    `json:"ref"`
	Checksum    string    `json:"checksum,omitempty"`
	SizeBytes   int64     `json:"size_bytes"`
	Reason      string    `json:"reason"`
	OperationID *string   `json:"operation_id,omitempty"`
	ActorUserID *string   `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	// Content is only populated by Get, never by List: a list of twenty config files
	// would otherwise carry a megabyte of payload nobody asked for.
	Content string `json:"content,omitempty"`
}

// PortRow is one listening socket owned by a connector object, as returned by
// GET /connectors/:id/ports.
type PortRow struct {
	Proto     string `json:"proto"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	Process   string `json:"process"`
	Ref       string `json:"ref"`
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

// LifecycleRequest is the body of POST /connectors/:id/actions.
type LifecycleRequest struct {
	Action string `json:"action"`
	Ref    string `json:"ref"`
}

// ConfigWriteRequest is the body of POST /connectors/:id/config.
type ConfigWriteRequest struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	BaseChecksum string `json:"base_checksum,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
}

// ConfigReadResponse is returned by GET /connectors/:id/config?path=...
type ConfigReadResponse struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Checksum string `json:"checksum"`
}
