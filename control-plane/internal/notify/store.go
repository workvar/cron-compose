package notify

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// ErrNotFound is returned when a target id doesn't exist.
var ErrNotFound = errors.New("target not found")

// Store wraps the notification_targets table.
type Store struct{ pool *pgxpool.Pool }

// NewStore wires a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const targetCols = `
  id, name, kind, coalesce(url,''), enabled, config, server_labels, on_statuses,
  last_error, last_fired_at, created_at
`

func scanTarget(row pgx.Row) (Target, error) {
	var t Target
	var cfg, labels, statuses []byte
	if err := row.Scan(&t.ID, &t.Name, &t.Kind, &t.URL, &t.Enabled,
		&cfg, &labels, &statuses, &t.LastError, &t.LastFiredAt, &t.CreatedAt); err != nil {
		return t, err
	}
	t.Config = map[string]string{}
	t.ServerLabels = map[string]string{}
	t.OnStatuses = []string{}
	_ = json.Unmarshal(cfg, &t.Config)
	_ = json.Unmarshal(labels, &t.ServerLabels)
	_ = json.Unmarshal(statuses, &t.OnStatuses)
	return t, nil
}

// List returns every target, newest first.
func (s *Store) List(ctx context.Context) ([]Target, error) {
	return s.query(ctx, `select `+targetCols+` from notification_targets order by created_at desc`)
}

// EnabledList returns only the enabled targets; this is what the fire path uses.
func (s *Store) EnabledList(ctx context.Context) ([]Target, error) {
	return s.query(ctx, `select `+targetCols+` from notification_targets where enabled = true`)
}

func (s *Store) query(ctx context.Context, sql string, args ...any) ([]Target, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Target{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one target, secrets included. Callers that hand it to a client must
// Redact it first.
func (s *Store) Get(ctx context.Context, id string) (Target, error) {
	t, err := scanTarget(s.pool.QueryRow(ctx,
		`select `+targetCols+` from notification_targets where id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Target{}, ErrNotFound
	}
	return t, err
}

// Insert persists a new target, enabled.
func (s *Store) Insert(ctx context.Context, in CreateInput) (Target, error) {
	id := ids.New()
	kind := in.Kind
	if kind == "" {
		kind = KindWebhook
	}
	_, err := s.pool.Exec(ctx, `
		insert into notification_targets
		  (id, name, kind, url, enabled, config, server_labels, on_statuses)
		values ($1, $2, $3, $4, true, $5, $6, $7)
	`, id, in.Name, kind, in.URL,
		mustJSONMap(in.Config), mustJSONMap(in.ServerLabels), mustJSONSlice(in.OnStatuses))
	if err != nil {
		return Target{}, err
	}
	return s.Get(ctx, id)
}

// Patch updates the provided fields only.
//
// A config value that comes back as the redaction placeholder is kept at its stored
// value rather than overwritten, so an edit form can submit the whole config object
// without blanking a password it was never shown.
func (s *Store) Patch(ctx context.Context, id string, in PatchInput) (Target, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return Target{}, err
	}
	if in.Name != nil {
		cur.Name = *in.Name
	}
	if in.URL != nil {
		cur.URL = *in.URL
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.Config != nil {
		merged := map[string]string{}
		for k, v := range *in.Config {
			if v == redactedPlaceholder {
				if old, hasKey := cur.Config[k]; hasKey {
					merged[k] = old
				}
				continue
			}
			merged[k] = v
		}
		cur.Config = merged
	}
	if in.ServerLabels != nil {
		cur.ServerLabels = *in.ServerLabels
	}
	if in.OnStatuses != nil {
		cur.OnStatuses = *in.OnStatuses
	}

	_, err = s.pool.Exec(ctx, `
		update notification_targets
		   set name = $2, url = $3, enabled = $4, config = $5,
		       server_labels = $6, on_statuses = $7
		 where id = $1
	`, id, cur.Name, cur.URL, cur.Enabled,
		mustJSONMap(cur.Config), mustJSONMap(cur.ServerLabels), mustJSONSlice(cur.OnStatuses))
	if err != nil {
		return Target{}, err
	}
	return s.Get(ctx, id)
}

// RecordDelivery stamps the outcome of the most recent attempt. An empty error means
// it worked, which is how the UI shows a target as healthy without a separate table.
func (s *Store) RecordDelivery(ctx context.Context, id, errMsg string) {
	_, _ = s.pool.Exec(ctx, `
		update notification_targets set last_error = $2, last_fired_at = now() where id = $1
	`, id, errMsg)
}

// Delete drops a target by id.
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from notification_targets where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// mustJSONMap marshals a map, never emitting SQL null: the columns are `not null`.
func mustJSONMap(m map[string]string) []byte {
	if m == nil {
		m = map[string]string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func mustJSONSlice(s []string) []byte {
	if s == nil {
		s = []string{}
	}
	b, err := json.Marshal(s)
	if err != nil {
		return []byte("[]")
	}
	return b
}
