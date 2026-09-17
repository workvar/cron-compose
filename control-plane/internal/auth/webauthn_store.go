package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cred mirrors a row in webauthn_credentials.
type Cred struct {
	ID              string
	UserID          string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	Transport       []string
	SignCount       uint32
	Name            string
	CreatedAt       time.Time
	LastUsedAt      *time.Time
}

// Challenge mirrors a row in webauthn_challenges.
type Challenge struct {
	ID        string
	UserID    *string
	Purpose   string
	Challenge []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

// ErrChallengeNotFound is returned when a challenge lookup misses.
var ErrChallengeNotFound = errors.New("webauthn challenge not found")

// WebAuthnStore is the data-access layer for passkey credentials and challenges.
type WebAuthnStore struct{ pool *pgxpool.Pool }

// NewWebAuthnStore wires a WebAuthnStore to a pgx pool.
func NewWebAuthnStore(pool *pgxpool.Pool) *WebAuthnStore {
	return &WebAuthnStore{pool: pool}
}

// InsertCredential stores a new passkey credential.
func (s *WebAuthnStore) InsertCredential(ctx context.Context, c Cred) error {
	transport := c.Transport
	if transport == nil {
		transport = []string{}
	}
	attestationType := c.AttestationType
	_, err := s.pool.Exec(ctx, `
		insert into webauthn_credentials (
			id, user_id, credential_id, public_key, attestation_type, transport, sign_count, name
		) values ($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.ID, c.UserID, c.CredentialID, c.PublicKey, attestationType, transport, c.SignCount, c.Name)
	return err
}

// ListByUser returns all credentials for a user, newest first.
func (s *WebAuthnStore) ListByUser(ctx context.Context, userID string) ([]Cred, error) {
	rows, err := s.pool.Query(ctx, `
		select id, user_id, credential_id, public_key, attestation_type, transport,
		       sign_count, name, created_at, last_used_at
		from webauthn_credentials
		where user_id = $1
		order by created_at desc
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Cred{}
	for rows.Next() {
		var c Cred
		var signCount int64
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey, &c.AttestationType, &c.Transport,
			&signCount, &c.Name, &c.CreatedAt, &c.LastUsedAt,
		); err != nil {
			return nil, err
		}
		c.SignCount = uint32(signCount)
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetByCredentialID returns a credential by its WebAuthn credential ID bytes.
func (s *WebAuthnStore) GetByCredentialID(ctx context.Context, id []byte) (*Cred, error) {
	var c Cred
	var signCount int64
	err := s.pool.QueryRow(ctx, `
		select id, user_id, credential_id, public_key, attestation_type, transport,
		       sign_count, name, created_at, last_used_at
		from webauthn_credentials
		where credential_id = $1
	`, id).Scan(
		&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey, &c.AttestationType, &c.Transport,
		&signCount, &c.Name, &c.CreatedAt, &c.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.SignCount = uint32(signCount)
	return &c, nil
}

// Delete removes a credential owned by the given user.
func (s *WebAuthnStore) Delete(ctx context.Context, userID, credPK string) error {
	tag, err := s.pool.Exec(ctx, `
		delete from webauthn_credentials where id = $1 and user_id = $2
	`, credPK, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateSignCount sets the signature counter for a credential.
func (s *WebAuthnStore) UpdateSignCount(ctx context.Context, credPK string, n uint32) error {
	tag, err := s.pool.Exec(ctx, `
		update webauthn_credentials
		set sign_count = $2, last_used_at = now()
		where id = $1
	`, credPK, n)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PutChallenge stores a single-use WebAuthn challenge.
func (s *WebAuthnStore) PutChallenge(ctx context.Context, ch Challenge) error {
	_, err := s.pool.Exec(ctx, `
		insert into webauthn_challenges (id, user_id, purpose, challenge, expires_at)
		values ($1, $2, $3, $4, $5)
	`, ch.ID, ch.UserID, ch.Purpose, ch.Challenge, ch.ExpiresAt)
	return err
}

// TakeChallenge returns a challenge and deletes it (single use).
func (s *WebAuthnStore) TakeChallenge(ctx context.Context, id string) (*Challenge, error) {
	var ch Challenge
	err := s.pool.QueryRow(ctx, `
		delete from webauthn_challenges
		where id = $1
		returning id, user_id, purpose, challenge, expires_at, created_at
	`, id).Scan(&ch.ID, &ch.UserID, &ch.Purpose, &ch.Challenge, &ch.ExpiresAt, &ch.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChallengeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}
