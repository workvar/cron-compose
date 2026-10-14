package notify

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// ErrPushNotFound is returned when a subscription id is missing or not owned.
var ErrPushNotFound = errors.New("push subscription not found")

// PushSubscription is one browser endpoint owned by a user.
type PushSubscription struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Endpoint   string    `json:"endpoint"`
	P256dh     string    `json:"-"`
	Auth       string    `json:"-"`
	UserAgent  string    `json:"user_agent,omitempty"`
	Events     []string  `json:"events"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// PushSubscribeInput is the body of POST /push/subscriptions.
type PushSubscribeInput struct {
	Endpoint  string   `json:"endpoint"`
	P256dh    string   `json:"p256dh"`
	Auth      string   `json:"auth"`
	UserAgent string   `json:"user_agent"`
	Events    []string `json:"events"`
}

type pushStore struct{ pool *pgxpool.Pool }

func newPushStore(pool *pgxpool.Pool) *pushStore { return &pushStore{pool: pool} }

func scanPush(row pgx.Row) (PushSubscription, error) {
	var s PushSubscription
	var events []byte
	if err := row.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth,
		&s.UserAgent, &events, &s.CreatedAt, &s.LastSeenAt); err != nil {
		return s, err
	}
	s.Events = []string{}
	_ = json.Unmarshal(events, &s.Events)
	return s, nil
}

func (s *pushStore) listByUser(ctx context.Context, userID string) ([]PushSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		select id, user_id, endpoint, p256dh, auth, user_agent, events, created_at, last_seen_at
		from push_subscriptions where user_id = $1 order by created_at desc
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PushSubscription{}
	for rows.Next() {
		item, err := scanPush(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pushStore) listForEvent(ctx context.Context, event string) ([]PushSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		select id, user_id, endpoint, p256dh, auth, user_agent, events, created_at, last_seen_at
		from push_subscriptions
		where events @> $1::jsonb
	`, jsonArray(event))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PushSubscription{}
	for rows.Next() {
		item, err := scanPush(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pushStore) upsert(ctx context.Context, userID string, in PushSubscribeInput) (PushSubscription, error) {
	events := normalizePushEvents(in.Events)
	raw, _ := json.Marshal(events)
	id := ids.New()
	_, err := s.pool.Exec(ctx, `
		insert into push_subscriptions (id, user_id, endpoint, p256dh, auth, user_agent, events, last_seen_at)
		values ($1, $2, $3, $4, $5, $6, $7::jsonb, now())
		on conflict (endpoint) do update set
		  user_id = excluded.user_id,
		  p256dh = excluded.p256dh,
		  auth = excluded.auth,
		  user_agent = excluded.user_agent,
		  events = excluded.events,
		  last_seen_at = now()
	`, id, userID, strings.TrimSpace(in.Endpoint), strings.TrimSpace(in.P256dh),
		strings.TrimSpace(in.Auth), strings.TrimSpace(in.UserAgent), string(raw))
	if err != nil {
		return PushSubscription{}, err
	}
	return scanPush(s.pool.QueryRow(ctx, `
		select id, user_id, endpoint, p256dh, auth, user_agent, events, created_at, last_seen_at
		from push_subscriptions where endpoint = $1
	`, strings.TrimSpace(in.Endpoint)))
}

func (s *pushStore) patchEvents(ctx context.Context, userID, id string, events []string) (PushSubscription, error) {
	events = normalizePushEvents(events)
	raw, _ := json.Marshal(events)
	t, err := scanPush(s.pool.QueryRow(ctx, `
		update push_subscriptions set events = $3::jsonb, last_seen_at = now()
		where id = $1 and user_id = $2
		returning id, user_id, endpoint, p256dh, auth, user_agent, events, created_at, last_seen_at
	`, id, userID, string(raw)))
	if errors.Is(err, pgx.ErrNoRows) {
		return PushSubscription{}, ErrPushNotFound
	}
	return t, err
}

func (s *pushStore) delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx,
		`delete from push_subscriptions where id = $1 and user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPushNotFound
	}
	return nil
}

func (s *pushStore) deleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := s.pool.Exec(ctx, `delete from push_subscriptions where endpoint = $1`, endpoint)
	return err
}

func normalizePushEvents(in []string) []string {
	if len(in) == 0 {
		return append([]string(nil), DefaultPushEvents...)
	}
	seen := map[string]bool{}
	out := []string{}
	for _, e := range in {
		e = strings.TrimSpace(e)
		if !ValidPushEvent(e) || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	if len(out) == 0 {
		return append([]string(nil), DefaultPushEvents...)
	}
	return out
}

func jsonArray(s string) string {
	b, _ := json.Marshal([]string{s})
	return string(b)
}
