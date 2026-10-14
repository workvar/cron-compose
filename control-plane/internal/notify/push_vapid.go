package notify

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VAPID holds the application server keypair used to authenticate Web Push.
type VAPID struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

type vapidCache struct {
	mu  sync.RWMutex
	got *VAPID
}

var vapidSingleton vapidCache

// EnsureVAPID loads keys from env, then the push_vapid row, or generates and
// persists a new pair. Safe to call on every request; the result is cached.
func EnsureVAPID(ctx context.Context, pool *pgxpool.Pool) (VAPID, error) {
	vapidSingleton.mu.RLock()
	if vapidSingleton.got != nil {
		v := *vapidSingleton.got
		vapidSingleton.mu.RUnlock()
		return v, nil
	}
	vapidSingleton.mu.RUnlock()

	vapidSingleton.mu.Lock()
	defer vapidSingleton.mu.Unlock()
	if vapidSingleton.got != nil {
		return *vapidSingleton.got, nil
	}

	if pub := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY")); pub != "" {
		priv := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
		if priv == "" {
			return VAPID{}, errors.New("VAPID_PUBLIC_KEY set but VAPID_PRIVATE_KEY is empty")
		}
		sub := strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))
		if sub == "" {
			sub = "mailto:croncompose@localhost"
		}
		v := VAPID{PublicKey: pub, PrivateKey: priv, Subject: sub}
		vapidSingleton.got = &v
		return v, nil
	}

	var v VAPID
	err := pool.QueryRow(ctx,
		`select public_key, private_key, subject from push_vapid where id = 1`).
		Scan(&v.PublicKey, &v.PrivateKey, &v.Subject)
	if err == nil {
		vapidSingleton.got = &v
		return v, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return VAPID{}, err
	}

	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return VAPID{}, err
	}
	sub := strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))
	if sub == "" {
		sub = "mailto:croncompose@localhost"
	}
	v = VAPID{PublicKey: pub, PrivateKey: priv, Subject: sub}
	_, err = pool.Exec(ctx, `
		insert into push_vapid (id, public_key, private_key, subject)
		values (1, $1, $2, $3)
		on conflict (id) do nothing
	`, v.PublicKey, v.PrivateKey, v.Subject)
	if err != nil {
		return VAPID{}, err
	}
	// Re-read in case another replica won the insert race.
	_ = pool.QueryRow(ctx,
		`select public_key, private_key, subject from push_vapid where id = 1`).
		Scan(&v.PublicKey, &v.PrivateKey, &v.Subject)
	vapidSingleton.got = &v
	return v, nil
}
