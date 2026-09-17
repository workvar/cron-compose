// Package servers owns the "server" entity: a target Linux machine running one agent.
package servers

import "time"

// Server matches the servers table in migrations/0001_init.sql.
type Server struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description,omitempty"`
	OS                 string            `json:"os,omitempty"`
	Arch               string            `json:"arch,omitempty"`
	Labels             map[string]string `json:"labels"`
	Status             string            `json:"status"`
	AgentVersion       string            `json:"agent_version,omitempty"`
	CertFingerprint    string            `json:"cert_fingerprint,omitempty"`
	LastSeenAt         *time.Time        `json:"last_seen_at,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	AgentRootEnabled   bool              `json:"agent_root_enabled"`
	AgentEuidRoot      bool              `json:"agent_euid_root"`
	AgentRootChangedAt *time.Time        `json:"agent_root_changed_at,omitempty"`
	AgentRootChangedBy *string           `json:"agent_root_changed_by,omitempty"`
	AgentServiceUser   string            `json:"agent_service_user,omitempty"`
}

// CreateInput is the body the API accepts to create a server.
type CreateInput struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// PatchInput allows editing rename/description/labels. Pointers distinguish
// "not provided" from "set to zero value".
type PatchInput struct {
	Name        *string            `json:"name,omitempty"`
	Description *string            `json:"description,omitempty"`
	Labels      *map[string]string `json:"labels,omitempty"`
}

// EnrollmentTokenResponse is returned alongside the created server. The token plaintext
// is shown exactly once; only its hash is persisted.
type EnrollmentTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateResponse is the wrapper returned by POST /servers.
type CreateResponse struct {
	Server         Server                  `json:"server"`
	Enrollment     EnrollmentTokenResponse `json:"enrollment"`
	InstallCommand string                  `json:"install_command"`
}
