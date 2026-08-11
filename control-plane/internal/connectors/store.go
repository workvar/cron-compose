package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a connector id does not exist.
var ErrNotFound = errors.New("connector not found")

// Store is the read-side data-access layer for connectors. Writes to the cache happen in
// the agent gateway (agentgw) when a ConnectorEvent arrives.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wires a Store to a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const cols = `
  id, server_id, kind, instance, coalesce(version,''), status, manageable,
  capabilities, config_paths, object_count, detail, last_seen_at, created_at
`

func scan(row pgx.Row, withServerName bool) (Connector, error) {
	var c Connector
	var caps, paths, detail []byte
	var lastSeen *time.Time
	dest := []any{
		&c.ID, &c.ServerID, &c.Kind, &c.Instance, &c.Version, &c.Status, &c.Manageable,
		&caps, &paths, &c.ObjectCount, &detail, &lastSeen, &c.CreatedAt,
	}
	if withServerName {
		dest = append(dest, &c.ServerName)
	}
	if err := row.Scan(dest...); err != nil {
		return c, err
	}
	c.LastSeenAt = lastSeen
	c.Capabilities = map[string]bool{}
	c.ConfigPaths = []string{}
	c.Detail = map[string]string{}
	if len(caps) > 0 {
		_ = json.Unmarshal(caps, &c.Capabilities)
	}
	if len(paths) > 0 {
		_ = json.Unmarshal(paths, &c.ConfigPaths)
	}
	if len(detail) > 0 {
		_ = json.Unmarshal(detail, &c.Detail)
	}
	return c, nil
}

// ListAll returns every connector across all servers, with the server name joined in,
// for the overview page.
func (s *Store) ListAll(ctx context.Context) ([]Connector, error) {
	// Columns are qualified to the connectors alias so the joined server name can be
	// appended; keep this in sync with cols above.
	q := `select c.id, c.server_id, c.kind, c.instance, coalesce(c.version,''), c.status,
	       c.manageable, c.capabilities, c.config_paths, c.object_count, c.detail,
	       c.last_seen_at, c.created_at, s.name
	     from connectors c join servers s on s.id = c.server_id
	     order by s.name, c.kind, c.instance`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query connectors: %w", err)
	}
	defer rows.Close()
	out := []Connector{}
	for rows.Next() {
		c, err := scan(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListByServer returns the connectors for one server.
func (s *Store) ListByServer(ctx context.Context, serverID string) ([]Connector, error) {
	q := `select ` + cols + ` from connectors where server_id = $1 order by kind, instance`
	rows, err := s.pool.Query(ctx, q, serverID)
	if err != nil {
		return nil, fmt.Errorf("query connectors: %w", err)
	}
	defer rows.Close()
	out := []Connector{}
	for rows.Next() {
		c, err := scan(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns one connector by id, or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Connector, error) {
	q := `select ` + cols + ` from connectors where id = $1`
	c, err := scan(s.pool.QueryRow(ctx, q, id), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return Connector{}, ErrNotFound
	}
	return c, err
}

// ListResources returns the cached config files and objects for a connector.
func (s *Store) ListResources(ctx context.Context, connectorID string) ([]Resource, error) {
	q := `select id, connector_id, type, ref, name, coalesce(state,''),
	         coalesce(checksum,''), coalesce(size_bytes,0), attributes, updated_at
	      from connector_resources where connector_id = $1
	      order by type, name`
	rows, err := s.pool.Query(ctx, q, connectorID)
	if err != nil {
		return nil, fmt.Errorf("query connector_resources: %w", err)
	}
	defer rows.Close()
	out := []Resource{}
	for rows.Next() {
		var r Resource
		var attrs []byte
		if err := rows.Scan(
			&r.ID, &r.ConnectorID, &r.Type, &r.Ref, &r.Name, &r.State,
			&r.Checksum, &r.SizeBytes, &attrs, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.Attributes = map[string]string{}
		if len(attrs) > 0 {
			_ = json.Unmarshal(attrs, &r.Attributes)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
