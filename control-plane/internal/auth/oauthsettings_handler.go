package auth

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
)

// AuditWriter is the subset of audit.Writer this package needs. Defined locally
// (rather than importing the audit package) because audit already imports auth for
// CurrentUserID, and Go doesn't allow the cycle; audit.PoolWriter satisfies this
// interface structurally, so callers just pass it straight through.
type AuditWriter interface {
	Write(ctx context.Context, actorUserID, action, targetType, targetID string, metadata map[string]any)
}

// RegisterOAuthSettings attaches the admin-only endpoints for configuring GitHub/
// GitLab OAuth app credentials from the Settings UI instead of .env. r must already
// be gated to admin (see api/router.go).
func RegisterOAuthSettings(r fiber.Router, store *OAuthSettingsStore, writer AuditWriter) {
	h := &oauthSettingsHandler{store: store, audit: writer}
	r.Get("/auth/oauth-settings", h.list)
	r.Put("/auth/oauth-settings/:provider", h.put)
	r.Delete("/auth/oauth-settings/:provider", h.clear)
}

type oauthSettingsHandler struct {
	store *OAuthSettingsStore
	audit AuditWriter
}

var errUnknownProvider = errors.New("provider must be github or gitlab")

func isKnownProvider(p string) bool { return p == "github" || p == "gitlab" }

func (h *oauthSettingsHandler) list(c fiber.Ctx) error {
	github, err := h.store.Get(c.Context(), "github")
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	gitlab, err := h.store.Get(c.Context(), "gitlab")
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(fiber.Map{"items": []OAuthSettings{github, gitlab}})
}

func (h *oauthSettingsHandler) put(c fiber.Ctx) error {
	provider := c.Params("provider")
	if !isKnownProvider(provider) {
		return jsonErr(c, fiber.StatusBadRequest, "bad_provider", errUnknownProvider)
	}
	var in OAuthSettingsInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonErr(c, fiber.StatusBadRequest, "bad_request", err)
	}
	out, err := h.store.Put(c.Context(), provider, in)
	if err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "save_failed", err)
	}
	if h.audit != nil {
		h.audit.Write(c.Context(), CurrentUserID(c), "oauth_settings.update", "oauth_provider", provider, map[string]any{
			"client_id_set": in.ClientID != "",
			"secret_set":    in.ClientSecret != "",
		})
	}
	return c.JSON(out)
}

func (h *oauthSettingsHandler) clear(c fiber.Ctx) error {
	provider := c.Params("provider")
	if !isKnownProvider(provider) {
		return jsonErr(c, fiber.StatusBadRequest, "bad_provider", errUnknownProvider)
	}
	if err := h.store.Clear(c.Context(), provider); err != nil {
		return jsonErr(c, fiber.StatusInternalServerError, "clear_failed", err)
	}
	if h.audit != nil {
		h.audit.Write(c.Context(), CurrentUserID(c), "oauth_settings.clear", "oauth_provider", provider, nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func jsonErr(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}
