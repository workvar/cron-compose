package deploys

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

type envRevealInput struct {
	ChallengeID string          `json:"challenge_id"`
	Credential  json.RawMessage `json:"credential"`
	App         string          `json:"app,omitempty"`
}

// revealEnv: POST /deploys/:id/env/reveal — passkey step-up, return plaintext env once.
func (h *handler) revealEnv(c fiber.Ctx) error {
	if h.stepUp == nil {
		return jsonError(c, fiber.StatusServiceUnavailable, "passkey_unavailable",
			errors.New("passkey step-up is not configured"))
	}
	ok, err := h.stepUp.HasPasskey(c.Context(), auth.CurrentUserID(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "passkey_lookup_failed", err)
	}
	if !ok {
		return jsonError(c, fiber.StatusForbidden, "passkey_required",
			errors.New("enroll a passkey before revealing deploy environment"))
	}
	var in envRevealInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.ChallengeID == "" {
		in.ChallengeID = c.Cookies(auth.ChallengeCookie)
	}
	if err := h.stepUp.VerifyStepUp(c.Context(), auth.CurrentUserID(c), in.ChallengeID, in.Credential); err != nil {
		return jsonError(c, fiber.StatusForbidden, "invalid_assertion", err)
	}

	p, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}

	type appEnv struct {
		Name string            `json:"name"`
		Env  map[string]string `json:"env"`
	}
	apps := []appEnv{}
	want := strings.TrimSpace(in.App)
	for _, a := range p.Apps {
		if want != "" && a.Name != want {
			continue
		}
		m, err := ResolveAppEnv(h.box, a)
		if err != nil {
			return jsonError(c, fiber.StatusInternalServerError, "decrypt_failed", err)
		}
		apps = append(apps, appEnv{Name: a.Name, Env: m})
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "deploy.env_reveal", "deploy", p.ID, map[string]any{
		"app": want,
	})
	return c.JSON(fiber.Map{"apps": apps})
}
