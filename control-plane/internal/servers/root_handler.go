package servers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

type agentRootInput struct {
	Enabled     bool            `json:"enabled"`
	ChallengeID string          `json:"challenge_id"`
	Credential  json.RawMessage `json:"credential"`
}

type passkeyChecker interface {
	HasPasskey(ctx context.Context, userID string) (bool, error)
}

type stepUpVerifier interface {
	VerifyStepUp(ctx context.Context, userID, challengeID string, assertion []byte) error
}

type agentRootStore interface {
	SetAgentRootEnabled(ctx context.Context, id string, enabled bool, changedBy string) (Server, error)
}

func (h *handler) setAgentRoot(c fiber.Ctx) error {
	ok, err := h.passkeys.HasPasskey(c.Context(), auth.CurrentUserID(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "passkey_lookup_failed", err)
	}
	if !ok {
		return jsonError(c, fiber.StatusForbidden, "passkey_required", errors.New("enroll a passkey before changing agent root access"))
	}
	var in agentRootInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.ChallengeID == "" {
		in.ChallengeID = c.Cookies(auth.ChallengeCookie)
	}
	if err := h.stepUp.VerifyStepUp(c.Context(), auth.CurrentUserID(c), in.ChallengeID, in.Credential); err != nil {
		return jsonError(c, fiber.StatusForbidden, "invalid_assertion", err)
	}
	userID := auth.CurrentUserID(c)
	srv, err := h.roots.SetAgentRootEnabled(c.Context(), c.Params("id"), in.Enabled, userID)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "update_failed", err)
	}
	action := "server.agent_root.disable"
	if in.Enabled {
		action = "server.agent_root.enable"
	}
	h.audit.Write(c.Context(), userID, action, "server", srv.ID, nil)
	return c.JSON(srv)
}
