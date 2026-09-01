package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	oidcStateCookie  = "cc_oidc_state"
	oidcReturnedTo   = "cc_oidc_returned_to"
	oidcCookieMaxAge = 10 * time.Minute
)

type oidcHandler struct {
	store    *Store
	secret   []byte
	ttl      time.Duration
	oidc     *OIDC
	postPath string // where to send the browser after a successful login
}

// RegisterOIDC attaches /auth/oidc/start and /auth/oidc/callback when SSO is enabled.
// Safe to call with o == nil; it just no-ops.
func RegisterOIDC(r fiber.Router, store *Store, secret []byte, o *OIDC, postPath string) {
	if o == nil {
		return
	}
	h := &oidcHandler{store: store, secret: secret, ttl: 7 * 24 * time.Hour, oidc: o, postPath: postPath}
	r.Get("/auth/oidc/start", h.start)
	r.Get("/auth/oidc/callback", h.callback)
}

// start redirects the browser to the OIDC provider with a fresh state.
func (h *oidcHandler) start(c fiber.Ctx) error {
	state, err := randomURLSafe(24)
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(oidcCookieMaxAge),
		HTTPOnly: true,
		SameSite: "Lax",
	})
	if to := c.Query("next"); to != "" {
		c.Cookie(&fiber.Cookie{
			Name:     oidcReturnedTo,
			Value:    to,
			Path:     "/",
			Expires:  time.Now().Add(oidcCookieMaxAge),
			HTTPOnly: true,
			SameSite: "Lax",
		})
	}
	return c.Redirect().To(h.oidc.OAuth.AuthCodeURL(state))
}

// callback exchanges the code for tokens, looks up or auto-provisions the user, and
// installs the session cookie.
func (h *oidcHandler) callback(c fiber.Ctx) error {
	if errMsg := c.Query("error"); errMsg != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "oidc_error", "message": errMsg},
		})
	}
	wantState := c.Cookies(oidcStateCookie)
	if wantState == "" || wantState != c.Query("state") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "bad_state", "message": "state mismatch"},
		})
	}
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "missing_code", "message": "missing code"},
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()

	tok, err := h.oidc.OAuth.Exchange(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "exchange_failed", "message": err.Error()},
		})
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "no_id_token", "message": "id_token missing"},
		})
	}
	idTok, err := h.oidc.Verifier.Verify(ctx, rawID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "bad_id_token", "message": err.Error()},
		})
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "bad_claims", "message": err.Error()},
		})
	}
	if claims.Email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "missing_email", "message": "id_token has no email claim"},
		})
	}

	u, err := h.findOrCreate(ctx, claims.Email, claims.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "provision_failed", "message": err.Error()},
		})
	}

	exp := time.Now().Add(h.ttl)
	value := SignSession(h.secret, Session{UserID: u.ID, ExpiresAt: exp})
	c.Cookie(&fiber.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		Expires:  exp,
		HTTPOnly: true,
		SameSite: "Lax",
	})
	// Clear ephemeral cookies.
	c.Cookie(&fiber.Cookie{Name: oidcStateCookie, Value: "", Path: "/", Expires: time.Unix(0, 0), HTTPOnly: true})

	to := c.Cookies(oidcReturnedTo)
	c.Cookie(&fiber.Cookie{Name: oidcReturnedTo, Value: "", Path: "/", Expires: time.Unix(0, 0), HTTPOnly: true})
	if to == "" || !looksSafeRedirect(to) {
		to = h.postPath
	}
	return c.Redirect().To(to)
}

// findOrCreate looks up the user by email; if missing, creates one with the configured
// default role.
func (h *oidcHandler) findOrCreate(ctx context.Context, email, name string) (User, error) {
	u, _, err := h.store.GetByEmailWithHash(ctx, email)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	if name == "" {
		name = email
	}
	// Empty password hash: SSO-only user. The password-login path rejects empty hashes.
	return h.store.Upsert(ctx, email, name, h.oidc.Cfg.DefaultRole, "")
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func looksSafeRedirect(to string) bool {
	if to == "" {
		return false
	}
	u, err := url.Parse(to)
	if err != nil {
		return false
	}
	// Only allow same-origin relative paths.
	return u.Scheme == "" && u.Host == "" && len(to) > 0 && to[0] == '/'
}

// Forces fmt to stay used in callback error formatting.
var _ = fmt.Sprintf
