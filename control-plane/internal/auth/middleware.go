package auth

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

const cookieName = "cc_session"

// roleRank assigns a numeric rank so RequireRole can do >= comparisons.
var roleRank = map[string]int{
	"viewer":   1,
	"operator": 2,
	"admin":    3,
	"owner":    4,
}

type ctxKey string

const (
	ctxUserID ctxKey = "user_id"
	ctxRole   ctxKey = "role"
)

func clearSession(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		SameSite: "Lax",
	})
}

func unauthenticated(c fiber.Ctx, message string) error {
	clearSession(c)
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": fiber.Map{"code": "unauthenticated", "message": message},
	})
}

// RequireAuth verifies the session cookie and attaches user_id + role into Locals.
// Unauthenticated requests get 401 and the stale cookie is cleared so the next
// visit can reach /login.
func RequireAuth(secret []byte, store *Store, log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		cookie := c.Cookies(cookieName)
		if cookie == "" {
			return unauthenticated(c, "missing session")
		}
		sess, err := ParseSession(secret, cookie)
		if err != nil {
			return unauthenticated(c, err.Error())
		}
		u, err := store.GetByID(c.Context(), sess.UserID)
		if err != nil {
			if log != nil && !errors.Is(err, ErrNotFound) {
				log.Warn("session user lookup failed", "user_id", sess.UserID, "err", err)
			}
			return unauthenticated(c, "unknown user")
		}
		c.Locals(ctxUserID, u.ID)
		c.Locals(ctxRole, u.Role)
		return c.Next()
	}
}

// RequireRole gates a route by minimum role rank. Use after RequireAuth.
func RequireRole(min string) fiber.Handler {
	wantRank := roleRank[min]
	return func(c fiber.Ctx) error {
		gotRank := roleRank[currentRole(c)]
		if gotRank < wantRank {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fiber.Map{"code": "forbidden", "message": "insufficient role"},
			})
		}
		return c.Next()
	}
}

// OptionalAuth attaches user_id + role when a valid session cookie is present, and
// otherwise continues. Used on routes that accept either a session or a bearer token.
func OptionalAuth(secret []byte, store *Store, log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		cookie := c.Cookies(cookieName)
		if cookie == "" {
			return c.Next()
		}
		sess, err := ParseSession(secret, cookie)
		if err != nil {
			return c.Next()
		}
		u, err := store.GetByID(c.Context(), sess.UserID)
		if err != nil {
			return c.Next()
		}
		c.Locals(ctxUserID, u.ID)
		c.Locals(ctxRole, u.Role)
		return c.Next()
	}
}

// CurrentUserID returns the authenticated user_id, or "" if not authenticated.
func CurrentUserID(c fiber.Ctx) string {
	if v, ok := c.Locals(ctxUserID).(string); ok {
		return v
	}
	return ""
}

// CurrentRole returns the authenticated role, or "" if not authenticated.
func CurrentRole(c fiber.Ctx) string { return currentRole(c) }

// HasMinRole reports whether the caller meets a role rank (false when unauthenticated).
func HasMinRole(c fiber.Ctx, min string) bool {
	return roleRank[currentRole(c)] >= roleRank[min]
}

func currentRole(c fiber.Ctx) string {
	if v, ok := c.Locals(ctxRole).(string); ok {
		return v
	}
	return ""
}
