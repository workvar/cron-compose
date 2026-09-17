package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

type passkeyHandler struct {
	log    *slog.Logger
	users  *Store
	store  *WebAuthnStore
	wa     *webauthn.WebAuthn
	secret []byte
	ttl    time.Duration
}

type passkeyFinishInput struct {
	ChallengeID string          `json:"challenge_id"`
	Name        string          `json:"name"`
	Credential  json.RawMessage `json:"credential"`
}

type passkeyView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// RegisterPasskeys attaches passwordless login (public) and enroll/list/delete
// (authenticated) routes when a Relying Party can be derived from publicURL.
func RegisterPasskeys(public, authed fiber.Router, log *slog.Logger, users *Store, waStore *WebAuthnStore, secret []byte, publicURL string) StepUp {
	wa, err := newWebAuthn(publicURL)
	if err != nil {
		if log != nil {
			log.Info("passkeys disabled", "err", err)
		}
		return disabledStepUp{}
	}
	h := &passkeyHandler{
		log: log, users: users, store: waStore, wa: wa, secret: secret, ttl: 7 * 24 * time.Hour,
	}
	public.Post("/auth/passkey/login/begin", h.loginBegin)
	public.Post("/auth/passkey/login/finish", h.loginFinish)
	authed.Post("/auth/passkey/register/begin", h.registerBegin)
	authed.Post("/auth/passkey/register/finish", h.registerFinish)
	authed.Post("/auth/passkey/step-up/begin", h.stepUpBegin)
	authed.Get("/auth/passkeys", h.list)
	authed.Delete("/auth/passkeys/:id", h.delete)
	return h
}

func (h *passkeyHandler) loginBegin(c fiber.Ctx) error {
	assertion, session, err := h.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "webauthn_failed", err)
	}
	chID, err := h.putSession(c, nil, purposeLogin, session)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "challenge_failed", err)
	}
	return c.JSON(fiber.Map{"publicKey": assertion.Response, "challenge_id": chID})
}

func (h *passkeyHandler) loginFinish(c fiber.Ctx) error {
	in, credJSON, err := parseFinish(c)
	if err != nil {
		return badRequest(c, "bad_request", err)
	}
	ch, err := takeLiveChallenge(c.Context(), h.store, in.ChallengeID)
	if err != nil {
		return challengeErr(c, err)
	}
	if ch.Purpose != purposeLogin {
		return badRequest(c, "bad_challenge", errors.New("challenge is not a login ceremony"))
	}
	session, err := decodeSession(ch.Challenge)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "challenge_failed", err)
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credJSON)
	if err != nil {
		return jsonErr(c, fiber.StatusUnauthorized, "invalid_assertion", err)
	}
	waUser, credential, err := h.wa.ValidatePasskeyLogin(h.discoverUser(c.Context()), *session, parsed)
	if err != nil {
		return jsonErr(c, fiber.StatusUnauthorized, "invalid_assertion", err)
	}
	wu, ok := waUser.(*webAuthnUser)
	if !ok {
		return jsonErr(c, fiber.StatusUnauthorized, "invalid_assertion", errors.New("unknown user"))
	}
	if err := h.touchCredential(c.Context(), credential); err != nil && h.log != nil {
		h.log.Warn("passkey sign count update failed", "err", err)
	}
	return h.issueSession(c, wu.user)
}

func (h *passkeyHandler) issueSession(c fiber.Ctx, u User) error {
	clearChallengeCookie(c)
	return issueSession(c, h.secret, u, h.ttl)
}

func (h *passkeyHandler) registerBegin(c fiber.Ctx) error {
	u, err := h.users.GetByID(c.Context(), CurrentUserID(c))
	if err != nil {
		return jsonErr(c, fiber.StatusUnauthorized, "unauthenticated", err)
	}
	wu, err := h.loadUser(c.Context(), u)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	exclusions := make([]protocol.CredentialDescriptor, len(wu.creds))
	for i, cred := range wu.creds {
		exclusions[i] = cred.Descriptor()
	}
	creation, session, err := h.wa.BeginRegistration(wu,
		webauthn.WithExclusions(exclusions),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "webauthn_failed", err)
	}
	userID := u.ID
	chID, err := h.putSession(c, &userID, purposeEnroll, session)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "challenge_failed", err)
	}
	return c.JSON(fiber.Map{"publicKey": creation.Response, "challenge_id": chID})
}

func (h *passkeyHandler) registerFinish(c fiber.Ctx) error {
	in, credJSON, err := parseFinish(c)
	if err != nil {
		return badRequest(c, "bad_request", err)
	}
	ch, err := takeLiveChallenge(c.Context(), h.store, in.ChallengeID)
	if err != nil {
		return challengeErr(c, err)
	}
	if ch.Purpose != purposeEnroll {
		return badRequest(c, "bad_challenge", errors.New("challenge is not an enroll ceremony"))
	}
	userID := CurrentUserID(c)
	if ch.UserID == nil || *ch.UserID != userID {
		return jsonErr(c, fiber.StatusForbidden, "forbidden", errors.New("challenge belongs to another user"))
	}
	u, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return jsonErr(c, fiber.StatusUnauthorized, "unauthenticated", err)
	}
	wu, err := h.loadUser(c.Context(), u)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	session, err := decodeSession(ch.Challenge)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "challenge_failed", err)
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credJSON)
	if err != nil {
		return badRequest(c, "invalid_attestation", err)
	}
	credential, err := h.wa.CreateCredential(wu, *session, parsed)
	if err != nil {
		return jsonErr(c, fiber.StatusUnauthorized, "invalid_attestation", err)
	}
	row, err := credFromWebAuthn(u.ID, in.Name, credential)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "encode_failed", err)
	}
	row.ID = ids.New()
	if err := h.store.InsertCredential(c.Context(), row); err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "insert_failed", err)
	}
	clearChallengeCookie(c)
	return c.JSON(passkeyView{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: time.Now(),
	})
}

func (h *passkeyHandler) list(c fiber.Ctx) error {
	items, err := h.store.ListByUser(c.Context(), CurrentUserID(c))
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	out := make([]passkeyView, 0, len(items))
	for _, item := range items {
		out = append(out, passkeyView{
			ID: item.ID, Name: item.Name, CreatedAt: item.CreatedAt, LastUsedAt: item.LastUsedAt,
		})
	}
	return c.JSON(fiber.Map{"items": out})
}

func (h *passkeyHandler) delete(c fiber.Ctx) error {
	err := h.store.Delete(c.Context(), CurrentUserID(c), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonErr(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *passkeyHandler) stepUpBegin(c fiber.Ctx) error {
	u, err := h.users.GetByID(c.Context(), CurrentUserID(c))
	if err != nil {
		return jsonErr(c, fiber.StatusUnauthorized, "unauthenticated", err)
	}
	ok, err := h.store.HasPasskey(c.Context(), u.ID)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	if !ok {
		return jsonErr(c, fiber.StatusForbidden, "passkey_required", errors.New("enroll a passkey before continuing"))
	}
	wu, err := h.loadUser(c.Context(), u)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	assertion, session, err := h.wa.BeginLogin(wu, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "webauthn_failed", err)
	}
	userID := u.ID
	chID, err := h.putSession(c, &userID, purposeStepUp, session)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "challenge_failed", err)
	}
	return c.JSON(fiber.Map{"publicKey": assertion.Response, "challenge_id": chID})
}

func (h *passkeyHandler) HasPasskey(ctx context.Context, userID string) (bool, error) {
	if h == nil || h.store == nil {
		return false, nil
	}
	return h.store.HasPasskey(ctx, userID)
}

func (h *passkeyHandler) VerifyStepUp(ctx context.Context, userID, challengeID string, assertion []byte) error {
	if h == nil || h.wa == nil || h.store == nil {
		return ErrPasskeyRequired
	}
	ch, err := takeLiveChallenge(ctx, h.store, challengeID)
	if err != nil {
		return err
	}
	if err := assertStepUpChallenge(ch, userID); err != nil {
		return err
	}
	session, err := decodeSession(ch.Challenge)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(assertion)
	if err != nil {
		return err
	}
	u, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	wu, err := h.loadUser(ctx, u)
	if err != nil {
		return err
	}
	if len(wu.creds) == 0 {
		return ErrPasskeyRequired
	}
	credential, err := h.wa.ValidateLogin(wu, *session, parsed)
	if err != nil {
		return err
	}
	if err := h.touchCredential(ctx, credential); err != nil && h.log != nil {
		h.log.Warn("passkey sign count update failed", "err", err)
	}
	return nil
}

func (h *passkeyHandler) loadUser(ctx context.Context, u User) (*webAuthnUser, error) {
	list, err := h.store.ListByUser(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, len(list))
	for i, c := range list {
		creds[i] = credFromRow(c)
	}
	return &webAuthnUser{user: u, creds: creds}, nil
}

func (h *passkeyHandler) discoverUser(ctx context.Context) webauthn.DiscoverableUserHandler {
	return func(rawID, userHandle []byte) (webauthn.User, error) {
		cred, err := h.store.GetByCredentialID(ctx, rawID)
		if err != nil {
			return nil, err
		}
		if len(userHandle) > 0 && string(userHandle) != cred.UserID {
			return nil, errors.New("user handle mismatch")
		}
		u, err := h.users.GetByID(ctx, cred.UserID)
		if err != nil {
			return nil, err
		}
		return h.loadUser(ctx, u)
	}
}

func (h *passkeyHandler) putSession(c fiber.Ctx, userID *string, purpose string, session *webauthn.SessionData) (string, error) {
	blob, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	chID := ids.New()
	exp := time.Now().Add(challengeTTL)
	if purpose == purposeStepUp {
		exp = time.Now().Add(stepUpChallengeTTL)
	}
	if err := h.store.PutChallenge(c.Context(), Challenge{
		ID: chID, UserID: userID, Purpose: purpose, Challenge: blob, ExpiresAt: exp,
	}); err != nil {
		return "", err
	}
	c.Cookie(&fiber.Cookie{
		Name:     ChallengeCookie,
		Value:    chID,
		Path:     "/",
		Expires:  exp,
		HTTPOnly: true,
		SameSite: "Lax",
	})
	return chID, nil
}

func (h *passkeyHandler) touchCredential(ctx context.Context, credential *webauthn.Credential) error {
	if credential == nil {
		return nil
	}
	stored, err := h.store.GetByCredentialID(ctx, credential.ID)
	if err != nil {
		return err
	}
	return h.store.UpdateSignCount(ctx, stored.ID, credential.Authenticator.SignCount)
}

func parseFinish(c fiber.Ctx) (passkeyFinishInput, []byte, error) {
	var in passkeyFinishInput
	_ = c.Bind().Body(&in)
	if in.ChallengeID == "" {
		in.ChallengeID = c.Cookies(ChallengeCookie)
	}
	cred := in.Credential
	if len(cred) == 0 || string(cred) == "null" {
		cred = append([]byte(nil), c.Body()...)
	}
	if in.ChallengeID == "" {
		return in, nil, errors.New("missing challenge_id")
	}
	if len(cred) == 0 {
		return in, nil, errors.New("missing credential")
	}
	return in, cred, nil
}

func decodeSession(b []byte) (*webauthn.SessionData, error) {
	var session webauthn.SessionData
	if err := json.Unmarshal(b, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func challengeErr(c fiber.Ctx, err error) error {
	if errors.Is(err, ErrChallengeExpired) {
		return jsonErr(c, fiber.StatusUnauthorized, "challenge_expired", err)
	}
	if errors.Is(err, ErrChallengeNotFound) {
		return jsonErr(c, fiber.StatusUnauthorized, "bad_challenge", err)
	}
	return jsonErr(c, fiber.StatusInternalServerError, "challenge_failed", err)
}

func clearChallengeCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     ChallengeCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		SameSite: "Lax",
	})
}
