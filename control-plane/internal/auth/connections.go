package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/singleflight"

	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// refreshSkew is how soon before token_expires_at we renew. Keeps a small buffer so
// a slow API call does not race the expiry clock.
const refreshSkew = 2 * time.Minute

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

// OAuthResolver returns the live OAuth app credentials for a provider name
// (github | gitlab). Used by Token() to refresh expired access tokens.
type OAuthResolver func(ctx context.Context, provider string) (OAuthProvider, error)

// ConnStore persists oauth identities and encrypted git tokens.
type ConnStore struct {
	pool    *pgxpool.Pool
	box     *cryptobox.Box
	http    *http.Client
	resolve OAuthResolver
	refresh singleflight.Group
}

// NewConnStore wires git connection storage. Call SetOAuthResolver so Token can
// renew expired access tokens; without it, Token returns the stored access token
// as-is (legacy behaviour).
func NewConnStore(pool *pgxpool.Pool, box *cryptobox.Box) *ConnStore {
	return &ConnStore{
		pool: pool,
		box:  box,
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// SetOAuthResolver wires the provider lookup used when refreshing tokens.
func (s *ConnStore) SetOAuthResolver(resolve OAuthResolver) {
	s.resolve = resolve
}

type storedGitToken struct {
	Access    string
	Refresh   string
	ExpiresAt time.Time // zero = unknown / non-expiring
}

// Upsert stores an encrypted access token (and optional refresh token) for a
// user+provider+purpose.
func (s *ConnStore) Upsert(ctx context.Context, userID, provider, purpose string, profile oauthProfile, tok oauthToken) error {
	accessBlob, err := s.box.Seal([]byte(tok.Access))
	if err != nil {
		return err
	}
	var refreshBlob []byte
	if tok.Refresh != "" {
		refreshBlob, err = s.box.Seal([]byte(tok.Refresh))
		if err != nil {
			return err
		}
	}
	var expiresAt *time.Time
	if !tok.ExpiresAt.IsZero() {
		t := tok.ExpiresAt.UTC()
		expiresAt = &t
	}
	id := ids.New()
	_, err = s.pool.Exec(ctx, `
		insert into git_connections (
		  id, user_id, provider, purpose, provider_user_id, login, email,
		  token_enc, refresh_token_enc, token_expires_at, updated_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		on conflict (user_id, provider, purpose) do update set
		  provider_user_id = excluded.provider_user_id,
		  login = excluded.login,
		  email = excluded.email,
		  token_enc = excluded.token_enc,
		  refresh_token_enc = excluded.refresh_token_enc,
		  token_expires_at = excluded.token_expires_at,
		  updated_at = now()
	`, id, userID, provider, purpose, profile.ID, profile.Login, profile.Email,
		accessBlob, refreshBlob, expiresAt)
	if err != nil {
		return err
	}
	return s.LinkIdentity(ctx, userID, provider, profile)
}

// Token decrypts the git-purpose access token for a provider, renewing it via
// the stored refresh token when it is near or past expiry. Concurrent callers for
// the same user+provider share one refresh via singleflight so a rotating IdP
// refresh token is not burned by parallel renewals.
func (s *ConnStore) Token(ctx context.Context, userID, provider string) (string, error) {
	tok, err := s.loadGitToken(ctx, userID, provider)
	if err != nil {
		return "", err
	}
	if !s.needsRefresh(tok) {
		return tok.Access, nil
	}
	key := userID + "\x00" + provider
	v, err, _ := s.refresh.Do(key, func() (any, error) {
		// Re-load: another flight may have already renewed while we waited.
		latest, err := s.loadGitToken(ctx, userID, provider)
		if err != nil {
			return "", err
		}
		if !s.needsRefresh(latest) {
			return latest.Access, nil
		}
		refreshed, err := s.refreshGitToken(ctx, provider, latest.Refresh)
		if err != nil {
			// Still inside the real lifetime: keep serving the current access token
			// rather than failing the request (transient IdP / race).
			if !latest.ExpiresAt.IsZero() && time.Now().Before(latest.ExpiresAt) {
				return latest.Access, nil
			}
			return "", fmt.Errorf("refresh %s token failed; reconnect git to renew the grant", provider)
		}
		// Providers sometimes omit refresh_token / expires_in on renewal; keep priors.
		if refreshed.Refresh == "" {
			refreshed.Refresh = latest.Refresh
		}
		if refreshed.ExpiresAt.IsZero() {
			refreshed.ExpiresAt = latest.ExpiresAt
		}
		if err := s.saveGitTokens(ctx, userID, provider, refreshed); err != nil {
			// IdP already rotated; return the new access so this request succeeds.
			// The next Token() soft-serves the old access until expiry, then asks
			// the operator to reconnect if the row never persisted.
			return refreshed.Access, nil
		}
		return refreshed.Access, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (s *ConnStore) needsRefresh(tok storedGitToken) bool {
	if tok.Refresh == "" || tok.ExpiresAt.IsZero() || s.resolve == nil {
		return false
	}
	return !time.Now().Before(tok.ExpiresAt.Add(-refreshSkew))
}

func (s *ConnStore) loadGitToken(ctx context.Context, userID, provider string) (storedGitToken, error) {
	var accessBlob, refreshBlob []byte
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `
		select token_enc, refresh_token_enc, token_expires_at from git_connections
		where user_id = $1 and provider = $2 and purpose = 'git'
	`, userID, provider).Scan(&accessBlob, &refreshBlob, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedGitToken{}, ErrNotFound
	}
	if err != nil {
		return storedGitToken{}, err
	}
	access, err := s.box.Open(accessBlob)
	if err != nil {
		return storedGitToken{}, err
	}
	out := storedGitToken{Access: string(access)}
	if len(refreshBlob) > 0 {
		refresh, err := s.box.Open(refreshBlob)
		if err != nil {
			return storedGitToken{}, err
		}
		out.Refresh = string(refresh)
	}
	if expiresAt != nil {
		out.ExpiresAt = expiresAt.UTC()
	}
	return out, nil
}

func (s *ConnStore) saveGitTokens(ctx context.Context, userID, provider string, tok oauthToken) error {
	accessBlob, err := s.box.Seal([]byte(tok.Access))
	if err != nil {
		return err
	}
	var refreshBlob []byte
	if tok.Refresh != "" {
		refreshBlob, err = s.box.Seal([]byte(tok.Refresh))
		if err != nil {
			return err
		}
	}
	var expiresAt *time.Time
	if !tok.ExpiresAt.IsZero() {
		t := tok.ExpiresAt.UTC()
		expiresAt = &t
	}
	tag, err := s.pool.Exec(ctx, `
		update git_connections
		set token_enc = $1, refresh_token_enc = $2, token_expires_at = $3, updated_at = now()
		where user_id = $4 and provider = $5 and purpose = 'git'
	`, accessBlob, refreshBlob, expiresAt, userID, provider)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ConnStore) refreshGitToken(ctx context.Context, provider, refresh string) (oauthToken, error) {
	p, err := s.resolve(ctx, provider)
	if err != nil {
		return oauthToken{}, err
	}
	if !p.Enabled() || p.ClientSecret == "" || p.TokenURL == "" {
		return oauthToken{}, errors.New("oauth provider is not configured")
	}
	form := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"refresh_token": {refresh},
		"grant_type":    {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthToken{}, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return oauthToken{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return oauthToken{}, fmt.Errorf("token http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return parseOAuthTokenResponse(body)
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
