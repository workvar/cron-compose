package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
)

// OAuthSettings is the admin-visible view of a provider's DB-stored OAuth config.
// The client secret itself is never returned, only whether one is set.
type OAuthSettings struct {
	Provider    string    `json:"provider"`
	ClientID    string    `json:"client_id"`
	HasSecret   bool      `json:"has_secret"`
	RedirectURL string    `json:"redirect_url"`
	BaseURL     string    `json:"base_url,omitempty"`
	Configured  bool      `json:"configured"` // true once this override is usable (has client_id + a secret)
	UpdatedAt   time.Time `json:"updated_at"`
}

// OAuthSettingsInput is what an admin PUTs to configure a provider. ClientSecret is
// optional on update: leaving it blank keeps whatever secret is already stored, so
// editing the client ID or redirect URL doesn't force re-pasting the secret.
type OAuthSettingsInput struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	BaseURL      string `json:"base_url"`
}

// OAuthSettingsStore reads and writes the oauth_settings table. It also resolves the
// live OAuthProvider a request should use: the DB override when one is configured,
// otherwise the env-derived default the control plane booted with.
type OAuthSettingsStore struct {
	pool *pgxpool.Pool
	box  *cryptobox.Box
}

// NewOAuthSettingsStore wires an OAuthSettingsStore.
func NewOAuthSettingsStore(pool *pgxpool.Pool, box *cryptobox.Box) *OAuthSettingsStore {
	return &OAuthSettingsStore{pool: pool, box: box}
}

type oauthSettingsRow struct {
	clientID     string
	clientSecret []byte
	redirectURL  string
	baseURL      string
	updatedAt    time.Time
}

func (s *OAuthSettingsStore) fetch(ctx context.Context, provider string) (oauthSettingsRow, bool, error) {
	var row oauthSettingsRow
	err := s.pool.QueryRow(ctx, `
		select client_id, client_secret_enc, redirect_url, base_url, updated_at
		from oauth_settings where provider = $1
	`, provider).Scan(&row.clientID, &row.clientSecret, &row.redirectURL, &row.baseURL, &row.updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauthSettingsRow{}, false, nil
	}
	if err != nil {
		return oauthSettingsRow{}, false, err
	}
	return row, true, nil
}

// Get returns the admin-visible view of one provider's stored config. It always
// returns a value (zeroed when nothing is stored yet), never ErrNotFound.
func (s *OAuthSettingsStore) Get(ctx context.Context, provider string) (OAuthSettings, error) {
	row, ok, err := s.fetch(ctx, provider)
	if err != nil {
		return OAuthSettings{}, err
	}
	if !ok {
		return OAuthSettings{Provider: provider}, nil
	}
	return OAuthSettings{
		Provider:    provider,
		ClientID:    row.clientID,
		HasSecret:   len(row.clientSecret) > 0,
		RedirectURL: row.redirectURL,
		BaseURL:     row.baseURL,
		Configured:  row.clientID != "" && len(row.clientSecret) > 0,
		UpdatedAt:   row.updatedAt,
	}, nil
}

// Put saves a provider's config. An empty ClientSecret keeps the previously stored
// secret (if any) rather than clearing it.
func (s *OAuthSettingsStore) Put(ctx context.Context, provider string, in OAuthSettingsInput) (OAuthSettings, error) {
	var secretBlob []byte
	if in.ClientSecret != "" {
		blob, err := s.box.Seal([]byte(in.ClientSecret))
		if err != nil {
			return OAuthSettings{}, err
		}
		secretBlob = blob
	} else if existing, ok, err := s.fetch(ctx, provider); err != nil {
		return OAuthSettings{}, err
	} else if ok {
		secretBlob = existing.clientSecret
	}

	_, err := s.pool.Exec(ctx, `
		insert into oauth_settings (provider, client_id, client_secret_enc, redirect_url, base_url, updated_at)
		values ($1, $2, $3, $4, $5, now())
		on conflict (provider) do update set
			client_id = excluded.client_id,
			client_secret_enc = excluded.client_secret_enc,
			redirect_url = excluded.redirect_url,
			base_url = excluded.base_url,
			updated_at = now()
	`, provider, in.ClientID, secretBlob, in.RedirectURL, in.BaseURL)
	if err != nil {
		return OAuthSettings{}, err
	}
	return s.Get(ctx, provider)
}

// Clear removes a provider's DB override, reverting it to whatever the control
// plane's env vars say (which may be "not configured" at all).
func (s *OAuthSettingsStore) Clear(ctx context.Context, provider string) error {
	_, err := s.pool.Exec(ctx, `delete from oauth_settings where provider = $1`, provider)
	return err
}

// Resolve returns the OAuthProvider a request should actually use: the DB override
// when it's usable (client_id + secret both set), otherwise envDefault untouched.
func (s *OAuthSettingsStore) Resolve(ctx context.Context, provider string, envDefault OAuthProvider) (OAuthProvider, error) {
	row, ok, err := s.fetch(ctx, provider)
	if err != nil {
		// A settings-store hiccup shouldn't take down login/connect entirely; fall
		// back to whatever the process booted with.
		return envDefault, nil //nolint:nilerr
	}
	if !ok || row.clientID == "" || len(row.clientSecret) == 0 {
		return envDefault, nil
	}
	secret, err := s.box.Open(row.clientSecret)
	if err != nil {
		return envDefault, nil
	}
	redirect := row.redirectURL
	if redirect == "" {
		redirect = envDefault.RedirectURL
	}
	if provider == "gitlab" {
		base := row.baseURL
		if base == "" {
			base = envDefault.APIBase // best-effort; GitLabProvider re-derives from base URL below
		}
		return GitLabProvider(row.clientID, string(secret), redirect, gitlabBaseFromRow(row, envDefault)), nil
	}
	return GitHubProvider(row.clientID, string(secret), redirect), nil
}

// gitlabBaseFromRow recovers the plain "https://gitlab.example.com" base URL GitLabProvider
// wants, preferring the row's explicit base_url and otherwise the env default's own base
// (derived from its AuthURL, since OAuthProvider doesn't carry the raw base separately).
func gitlabBaseFromRow(row oauthSettingsRow, envDefault OAuthProvider) string {
	if row.baseURL != "" {
		return row.baseURL
	}
	const suffix = "/oauth/authorize"
	if len(envDefault.AuthURL) > len(suffix) {
		return envDefault.AuthURL[:len(envDefault.AuthURL)-len(suffix)]
	}
	return ""
}
