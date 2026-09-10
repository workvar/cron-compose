package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// GitConnection is a linked GitHub/GitLab grant (never includes the raw token).
type GitConnection struct {
	ID             string    `json:"id"`
	Provider       string    `json:"provider"`
	Purpose        string    `json:"purpose"`
	Login          string    `json:"login"`
	Email          string    `json:"email"`
	ProviderUserID string    `json:"provider_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConnStore persists oauth identities and encrypted git tokens.
type ConnStore struct {
	pool *pgxpool.Pool
	box  *cryptobox.Box
}

// NewConnStore wires git connection storage.
func NewConnStore(pool *pgxpool.Pool, box *cryptobox.Box) *ConnStore {
	return &ConnStore{pool: pool, box: box}
}

// Upsert stores an encrypted access token for a user+provider+purpose.
func (s *ConnStore) Upsert(ctx context.Context, userID, provider, purpose string, profile oauthProfile, token string) error {
	blob, err := s.box.Seal([]byte(token))
	if err != nil {
		return err
	}
	id := ids.New()
	_, err = s.pool.Exec(ctx, `
		insert into git_connections (id, user_id, provider, purpose, provider_user_id, login, email, token_enc, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, now())
		on conflict (user_id, provider, purpose) do update set
		  provider_user_id = excluded.provider_user_id,
		  login = excluded.login,
		  email = excluded.email,
		  token_enc = excluded.token_enc,
		  updated_at = now()
	`, id, userID, provider, purpose, profile.ID, profile.Login, profile.Email, blob)
	if err != nil {
		return err
	}
	return s.LinkIdentity(ctx, userID, provider, profile)
}

// Token decrypts the git-purpose token for a provider.
func (s *ConnStore) Token(ctx context.Context, userID, provider string) (string, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `
		select token_enc from git_connections
		where user_id = $1 and provider = $2 and purpose = 'git'
	`, userID, provider).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	plain, err := s.box.Open(blob)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// ListGit returns linked git grants (not login-only rows).
func (s *ConnStore) ListGit(ctx context.Context, userID string) ([]GitConnection, error) {
	rows, err := s.pool.Query(ctx, `
		select id, provider, purpose, login, email, provider_user_id, created_at
		from git_connections where user_id = $1 and purpose = 'git'
		order by provider
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GitConnection{}
	for rows.Next() {
		var g GitConnection
		if err := rows.Scan(&g.ID, &g.Provider, &g.Purpose, &g.Login, &g.Email, &g.ProviderUserID, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeleteGit drops a linked git grant.
func (s *ConnStore) DeleteGit(ctx context.Context, userID, provider string) error {
	_, err := s.pool.Exec(ctx, `
		delete from git_connections where user_id = $1 and provider = $2 and purpose = 'git'
	`, userID, provider)
	return err
}

// LinkIdentity records which provider account maps to a CronCompose user.
func (s *ConnStore) LinkIdentity(ctx context.Context, userID, provider string, profile oauthProfile) error {
	_, err := s.pool.Exec(ctx, `
		insert into oauth_identities (provider, provider_user_id, user_id, login, email)
		values ($1, $2, $3, $4, $5)
		on conflict (provider, provider_user_id) do update set
		  user_id = excluded.user_id,
		  login = excluded.login,
		  email = excluded.email
	`, provider, profile.ID, userID, profile.Login, profile.Email)
	return err
}

// UserForIdentity looks up a user by provider account.
func (s *ConnStore) UserForIdentity(ctx context.Context, provider, providerUserID string) (string, error) {
	var uid string
	err := s.pool.QueryRow(ctx, `
		select user_id from oauth_identities where provider = $1 and provider_user_id = $2
	`, provider, providerUserID).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return uid, err
}
