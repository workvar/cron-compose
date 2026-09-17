package servers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a server id does not exist.
var ErrNotFound = errors.New("server not found")

// Store is the data-access layer for servers.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wires a Store to a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const selectColumns = `
  id, name, coalesce(description,''), coalesce(os,''), coalesce(arch,''),
  labels, status, coalesce(agent_version,''), coalesce(cert_fingerprint,''),
  last_seen_at, created_at,
  agent_root_enabled, agent_euid_root, agent_root_changed_at, agent_root_changed_by,
  coalesce(agent_service_user,'')
`

func scan(row pgx.Row) (Server, error) {
	var s Server
	var labelsRaw []byte
	var lastSeen *time.Time
	if err := row.Scan(
		&s.ID, &s.Name, &s.Description, &s.OS, &s.Arch,
		&labelsRaw, &s.Status, &s.AgentVersion, &s.CertFingerprint,
		&lastSeen, &s.CreatedAt,
		&s.AgentRootEnabled, &s.AgentEuidRoot, &s.AgentRootChangedAt, &s.AgentRootChangedBy,
		&s.AgentServiceUser,
	); err != nil {
		return s, err
	}
	s.LastSeenAt = lastSeen
	if len(labelsRaw) > 0 {
		_ = json.Unmarshal(labelsRaw, &s.Labels)
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s, nil
}

// List returns every server. Pagination is a Phase 2 concern.
func (s *Store) List(ctx context.Context) ([]Server, error) {
	q := `select ` + selectColumns + ` from servers order by created_at desc`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query servers: %w", err)
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		srv, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// Get returns a server by id or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Server, error) {
	q := `select ` + selectColumns + ` from servers where id = $1`
	srv, err := scan(s.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	return srv, err
}

// Insert persists a fresh server row.
func (s *Store) Insert(ctx context.Context, srv Server) error {
	labels, _ := json.Marshal(srv.Labels)
	_, err := s.pool.Exec(ctx, `
		insert into servers (id, name, description, labels, status, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, srv.ID, srv.Name, srv.Description, labels, srv.Status, srv.CreatedAt)
	return err
}

// Patch applies only the non-nil fields of in to the row.
func (s *Store) Patch(ctx context.Context, id string, in PatchInput) (Server, error) {
	// Build the update dynamically but stay simple: load, mutate, write back.
	srv, err := s.Get(ctx, id)
	if err != nil {
		return Server{}, err
	}
	if in.Name != nil {
		srv.Name = *in.Name
	}
	if in.Description != nil {
		srv.Description = *in.Description
	}
	if in.Labels != nil {
		srv.Labels = *in.Labels
	}
	labels, _ := json.Marshal(srv.Labels)
	_, err = s.pool.Exec(ctx, `
		update servers set name = $1, description = $2, labels = $3
		where id = $4
	`, srv.Name, srv.Description, labels, id)
	if err != nil {
		return Server{}, err
	}
	return srv, nil
}

// Delete removes a server (cascades to its jobs and runs).
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from servers where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAgentRootEnabled persists the desired agent-root flag and who changed it.
// The HTTP handler pushes AgentRootCommand after this write.
func (s *Store) SetAgentRootEnabled(ctx context.Context, id string, enabled bool, changedBy string) (Server, error) {
	tag, err := s.pool.Exec(ctx, `
		update servers
		set agent_root_enabled = $2,
		    agent_root_changed_at = now(),
		    agent_root_changed_by = $3
		where id = $1
	`, id, enabled, changedBy)
	if err != nil {
		return Server{}, err
	}
	if tag.RowsAffected() == 0 {
		return Server{}, ErrNotFound
	}
	return s.Get(ctx, id)
}
