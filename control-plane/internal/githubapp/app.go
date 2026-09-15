// Package githubapp authenticates as a GitHub App.
//
// A GitHub App is worth the setup over the OAuth token CronCompose already stores for
// a user, for three reasons. Its tokens are short-lived and scoped to the repos the
// App was installed on, rather than long-lived and scoped to everything the user can
// reach. It keeps working when the person who imported a repo leaves or revokes their
// grant. And it is the only way to write commit statuses that show up as CronCompose
// rather than as that user.
//
// The flow is two hops: sign a short JWT with the App's private key, exchange it for
// an installation access token, use that token as a normal bearer for the REST API.
// Installation tokens last an hour, so they are cached.
package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// jwtLifetime is under GitHub's 10 minute ceiling, with room for clock skew on both
// ends. GitHub rejects a JWT whose iat is in the future, hence the backdated iat.
const jwtLifetime = 9 * time.Minute

// App holds the App's identity and its token caches. The zero value is not usable;
// New returns nil when the App is not configured, and every caller treats a nil *App
// as "not set up", so this stays entirely optional.
type App struct {
	appID   string
	key     *rsa.PrivateKey
	apiBase string
	http    *http.Client

	mu       sync.Mutex
	tokens   map[int64]cachedToken // installation id -> access token
	installs map[string]int64      // "owner/repo" -> installation id
}

type cachedToken struct {
	token   string
	expires time.Time
}

// New builds an App from the id and PEM private key. Both empty means the App is not
// configured, which is not an error: New returns (nil, nil) and the features that
// need it stay off. A malformed key is an error, because that is a typo in config
// rather than a deliberate opt-out.
func New(appID string, pemKey []byte, apiBase string) (*App, error) {
	if appID == "" && len(pemKey) == 0 {
		return nil, nil
	}
	if appID == "" || len(pemKey) == 0 {
		return nil, errors.New("github app needs both an app id and a private key")
	}
	key, err := parseKey(pemKey)
	if err != nil {
		return nil, err
	}
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &App{
		appID:    appID,
		key:      key,
		apiBase:  apiBase,
		http:     &http.Client{Timeout: 20 * time.Second},
		tokens:   map[int64]cachedToken{},
		installs: map[string]int64{},
	}, nil
}

// Configured reports whether there is an App to use. Safe on a nil receiver so
// callers can hold a possibly-nil *App without guarding every use.
func (a *App) Configured() bool { return a != nil }

func parseKey(pemKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, errors.New("github app private key is not PEM")
	}
	// GitHub hands out PKCS#1 ("RSA PRIVATE KEY"); some tooling re-wraps it as PKCS#8.
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("github app private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app private key must be RSA")
	}
	return key, nil
}

// appJWT mints the short-lived assertion that identifies the App itself. Hand-rolled
// rather than pulling in a JWT library: this is one algorithm, one claim set, and the
// signing is nine lines of stdlib.
func (a *App) appJWT(now time.Time) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(), // tolerate our clock running fast
		"exp": now.Add(jwtLifetime).Unix(),
		"iss": a.appID,
	}
	segments, err := encodeSegments(header, claims)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(segments))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return segments + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func encodeSegments(header map[string]string, claims map[string]any) (string, error) {
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c), nil
}

// InstallationToken returns a bearer token for one installation, minting a new one
// when the cached token is within a minute of expiring.
func (a *App) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	a.mu.Lock()
	cached, ok := a.tokens[installationID]
	a.mu.Unlock()
	if ok && time.Until(cached.expires) > time.Minute {
		return cached.token, nil
	}

	assertion, err := a.appJWT(time.Now())
	if err != nil {
		return "", err
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	url := a.apiBase + "/app/installations/" + strconv.FormatInt(installationID, 10) + "/access_tokens"
	if err := a.do(ctx, http.MethodPost, url, assertion, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("github app: empty installation token")
	}
	a.mu.Lock()
	a.tokens[installationID] = cachedToken{token: out.Token, expires: out.ExpiresAt}
	a.mu.Unlock()
	return out.Token, nil
}
