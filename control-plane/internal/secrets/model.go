// Package secrets owns encrypted, write-only key/value pairs that jobs reference by
// name. Values are never returned by any endpoint.
package secrets

import "time"

// Secret is the read shape, without the value.
type Secret struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"` // global | server | job
	ScopeID   string    `json:"scope_id,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInput is the body of POST /secrets.
type CreateInput struct {
	Scope   string `json:"scope"` // optional, defaults to "global"
	ScopeID string `json:"scope_id,omitempty"`
	Name    string `json:"name"`
	Value   string `json:"value"` // plaintext, write-only, encrypted at rest
}
