package connectors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

func normalizeProto(proto string) string {
	p := strings.ToLower(strings.TrimSpace(proto))
	if p == "" {
		return "tcp"
	}
	return p
}

// ListLabelsForServer returns every stored port label on one server.
func (s *Store) ListLabelsForServer(ctx context.Context, serverID string) ([]PortLabel, error) {
	rows, err := s.pool.Query(ctx, `
		select id, server_id, proto, address, port, label, updated_at
		  from port_labels
		 where server_id = $1
		 order by port, address
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("query port labels: %w", err)
	}
	defer rows.Close()
	return scanPortLabels(rows)
}

// ListAllLabels returns every stored port label, for the Ports overview.
func (s *Store) ListAllLabels(ctx context.Context) ([]PortLabel, error) {
	rows, err := s.pool.Query(ctx, `
		select id, server_id, proto, address, port, label, updated_at
		  from port_labels
		 order by server_id, port, address
	`)
	if err != nil {
		return nil, fmt.Errorf("query port labels: %w", err)
	}
	defer rows.Close()
	return scanPortLabels(rows)
}

// UpsertLabel writes or replaces a label for one bind tuple.
func (s *Store) UpsertLabel(ctx context.Context, serverID, proto, address string, port int, label string) (PortLabel, error) {
	id := ids.New()
	proto = normalizeProto(proto)
	var out PortLabel
	err := s.pool.QueryRow(ctx, `
		insert into port_labels (id, server_id, proto, address, port, label)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (server_id, proto, address, port)
		do update set label = excluded.label, updated_at = now()
		returning id, server_id, proto, address, port, label, updated_at
	`, id, serverID, proto, address, port, label).Scan(
		&out.ID, &out.ServerID, &out.Proto, &out.Address, &out.Port, &out.Label, &out.UpdatedAt,
	)
	return out, err
}

// DeleteLabel removes a label for one bind tuple. Missing rows are not an error.
func (s *Store) DeleteLabel(ctx context.Context, serverID, proto, address string, port int) error {
	_, err := s.pool.Exec(ctx, `
		delete from port_labels
		 where server_id = $1 and proto = $2 and address = $3 and port = $4
	`, serverID, normalizeProto(proto), address, port)
	return err
}

type portLabelRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanPortLabels(rows portLabelRows) ([]PortLabel, error) {
	out := []PortLabel{}
	for rows.Next() {
		var l PortLabel
		var updated time.Time
		if err := rows.Scan(&l.ID, &l.ServerID, &l.Proto, &l.Address, &l.Port, &l.Label, &updated); err != nil {
			return nil, err
		}
		l.UpdatedAt = updated
		out = append(out, l)
	}
	return out, rows.Err()
}
