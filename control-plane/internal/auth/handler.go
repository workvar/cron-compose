package auth

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

type handler struct {
	log         *slog.Logger
	store       *Store
	secret      []byte
	ttl         time.Duration
	oidcEnabled bool
}

func (h *handler) config(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"password_login": true,
		"oidc_enabled":   h.oidcEnabled,
		"oidc_start_url": "/api/v1/auth/oidc/start",
	})
}

// Register attaches the endpoints that must work before a session exists:
// /auth/login, /auth/logout, /auth/config.
//
// /me is deliberately NOT here. It reads the caller's identity out of the request
// locals, which only RequireAuth populates, so it has to be registered on the
// authenticated group instead. See RegisterMe.
func Register(r fiber.Router, log *slog.Logger, store *Store, secret []byte, oidcEnabled bool) {
	h := newHandler(log, store, secret, oidcEnabled)
	r.Post("/auth/login", h.login)
	r.Post("/auth/logout", h.logout)
	r.Get("/auth/config", h.config)
}

// RegisterMe attaches /me to the authenticated group.
//
// This is a separate call because the two groups share a path prefix: registering /me
// on the public group would shadow the authenticated one, and the handler would then
// see an empty user id on every request.
func RegisterMe(r fiber.Router, log *slog.Logger, store *Store, secret []byte, oidcEnabled bool) {
	h := newHandler(log, store, secret, oidcEnabled)
	r.Get("/me", h.me)
}

func newHandler(log *slog.Logger, store *Store, secret []byte, oidcEnabled bool) *handler {
	return &handler{log: log, store: store, secret: secret, ttl: 7 * 24 * time.Hour, oidcEnabled: oidcEnabled}
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *handler) login(c fiber.Ctx) error {
	var in loginInput
	if err := c.Bind().Body(&in); err != nil {
		return badRequest(c, "bad_request", err)
	}
	if in.Email == "" || in.Password == "" {
		return badRequest(c, "missing_fields", errors.New("email and password are required"))
	}
	u, hash, err := h.store.GetByEmailWithHash(c.Context(), in.Email)
	if err != nil || hash == "" || !Verify(hash, in.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "invalid_credentials", "message": "wrong email or password"},
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
		Secure:   false, // dev only; set true behind TLS
		SameSite: "Lax",
	})
	return c.JSON(u)
}

func (h *handler) logout(c fiber.Ctx) error {
	clearSession(c)
	return c.SendStatus(fiber.StatusNoContent)
}

// me is mounted under the authenticated group so a 200 here doubles as a quick session
// check from the UI.
func (h *handler) me(c fiber.Ctx) error {
	u, err := h.store.GetByID(c.Context(), CurrentUserID(c))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "unauthenticated", "message": err.Error()},
		})
	}
	return c.JSON(u)
}

func badRequest(c fiber.Ctx, code string, err error) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}
