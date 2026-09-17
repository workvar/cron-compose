package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	purposeEnroll      = "enroll"
	purposeLogin       = "login"
	challengeTTL       = 5 * time.Minute
	challengeCookie    = "cc_webauthn"
	rpDisplayName      = "CronCompose"
	defaultPasskeyName = "Passkey"
)

// ErrChallengeExpired is returned when a challenge is taken after ExpiresAt.
var ErrChallengeExpired = errors.New("webauthn challenge expired")

// relyingPartyInfo is the WebAuthn RP ID (hostname) and allowed origins derived
// from PUBLIC_BASE_URL / PUBLIC_HTTP_URL.
type relyingPartyInfo struct {
	ID      string
	Origins []string
}

// relyingParty parses a public URL into an RP ID (hostname only) and the full
// origin used as the WebAuthn allowed origin.
func relyingParty(publicURL string) (relyingPartyInfo, error) {
	raw := strings.TrimSpace(publicURL)
	if raw == "" {
		return relyingPartyInfo{}, fmt.Errorf("public URL is empty")
	}
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return relyingPartyInfo{}, fmt.Errorf("invalid public URL %q: %w", publicURL, err)
	}
	if u.Hostname() == "" {
		return relyingPartyInfo{}, fmt.Errorf("invalid public URL %q: missing host", publicURL)
	}
	origin := u.Scheme + "://" + u.Host
	if u.Scheme == "" || origin == "://"+u.Host {
		return relyingPartyInfo{}, fmt.Errorf("invalid public URL %q: missing scheme", publicURL)
	}
	return relyingPartyInfo{ID: u.Hostname(), Origins: []string{origin}}, nil
}

func rejectExpiredChallenge(ch *Challenge) error {
	if ch == nil || !time.Now().Before(ch.ExpiresAt) {
		return ErrChallengeExpired
	}
	return nil
}

func takeLiveChallenge(ctx context.Context, store *WebAuthnStore, id string) (*Challenge, error) {
	ch, err := store.TakeChallenge(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := rejectExpiredChallenge(ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func newWebAuthn(publicURL string) (*webauthn.WebAuthn, error) {
	rp, err := relyingParty(publicURL)
	if err != nil {
		return nil, err
	}
	return webauthn.New(&webauthn.Config{
		RPID:          rp.ID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     rp.Origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementPreferred,
			RequireResidentKey: protocol.ResidentKeyNotRequired(),
			UserVerification:   protocol.VerificationPreferred,
		},
	})
}

type webAuthnUser struct {
	user  User
	creds []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte { return []byte(u.user.ID) }
func (u *webAuthnUser) WebAuthnName() string {
	return u.user.Email
}
func (u *webAuthnUser) WebAuthnDisplayName() string {
	if u.user.Name != "" {
		return u.user.Name
	}
	return u.user.Email
}
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func loadWebAuthnCred(c Cred) webauthn.Credential {
	var wa webauthn.Credential
	if json.Unmarshal(c.PublicKey, &wa) == nil && len(wa.ID) > 0 && len(wa.PublicKey) > 0 {
		wa.Authenticator.SignCount = c.SignCount
		return wa
	}
	transports := make([]protocol.AuthenticatorTransport, len(c.Transport))
	for i, t := range c.Transport {
		transports[i] = protocol.AuthenticatorTransport(t)
	}
	return webauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transport:       transports,
		Authenticator:   webauthn.Authenticator{SignCount: c.SignCount},
	}
}

func credFromWebAuthn(userID, name string, credential *webauthn.Credential) (Cred, error) {
	// Persist the full library credential (flags, attestation, COSE key) as JSON in
	// public_key so login can restore BackupEligible and related fields without a
	// schema change. SignCount is also stored in its column and overlaid on load.
	blob, err := json.Marshal(credential)
	if err != nil {
		return Cred{}, err
	}
	transports := make([]string, len(credential.Transport))
	for i, t := range credential.Transport {
		transports[i] = string(t)
	}
	if name == "" {
		name = defaultPasskeyName
	}
	return Cred{
		UserID:          userID,
		CredentialID:    credential.ID,
		PublicKey:       blob,
		AttestationType: credential.AttestationType,
		Transport:       transports,
		SignCount:       credential.Authenticator.SignCount,
		Name:            name,
	}, nil
}
