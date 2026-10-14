package notify

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

type pushHandler struct {
	log   *slog.Logger
	pool  *pgxpool.Pool
	store *pushStore
}

// RegisterPush attaches /push/* for any signed-in user. Browser push is personal
// (per device), unlike admin-only Slack/email targets.
func RegisterPush(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool) {
	h := &pushHandler{log: log, pool: pool, store: newPushStore(pool)}
	r.Get("/push/vapid-public-key", h.vapidPublic)
	r.Get("/push/subscriptions", h.list)
	r.Post("/push/subscriptions", h.subscribe)
	r.Patch("/push/subscriptions/:id", h.patch)
	r.Delete("/push/subscriptions/:id", h.delete)
	r.Post("/push/test", h.test)
}

func (h *pushHandler) vapidPublic(c fiber.Ctx) error {
	v, err := EnsureVAPID(c.Context(), h.pool)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "vapid_unavailable", err)
	}
	return c.JSON(fiber.Map{
		"public_key": v.PublicKey,
		"events":     DefaultPushEvents,
	})
}

func (h *pushHandler) list(c fiber.Ctx) error {
	uid := auth.CurrentUserID(c)
	items, err := h.store.listByUser(c.Context(), uid)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *pushHandler) subscribe(c fiber.Ctx) error {
	var in PushSubscribeInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.P256dh = strings.TrimSpace(in.P256dh)
	in.Auth = strings.TrimSpace(in.Auth)
	if in.Endpoint == "" || in.P256dh == "" || in.Auth == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields",
			errors.New("endpoint, p256dh, and auth are required"))
	}
	if in.UserAgent == "" {
		in.UserAgent = string(c.Request().Header.UserAgent())
	}
	uid := auth.CurrentUserID(c)
	sub, err := h.store.upsert(c.Context(), uid, in)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "save_failed", err)
	}
	return c.Status(fiber.StatusCreated).JSON(sub)
}

func (h *pushHandler) patch(c fiber.Ctx) error {
	var in struct {
		Events []string `json:"events"`
	}
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	sub, err := h.store.patchEvents(c.Context(), auth.CurrentUserID(c), c.Params("id"), in.Events)
	if errors.Is(err, ErrPushNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "patch_failed", err)
	}
	return c.JSON(sub)
}

func (h *pushHandler) delete(c fiber.Ctx) error {
	if err := h.store.delete(c.Context(), auth.CurrentUserID(c), c.Params("id")); err != nil {
		if errors.Is(err, ErrPushNotFound) {
			return jsonError(c, fiber.StatusNotFound, "not_found", err)
		}
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *pushHandler) test(c fiber.Ctx) error {
	uid := auth.CurrentUserID(c)
	items, err := h.store.listByUser(c.Context(), uid)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	if len(items) == 0 {
		return jsonError(c, fiber.StatusConflict, "no_subscription",
			errors.New("enable browser notifications on this device first"))
	}
	// Send only to this user's subscriptions via a one-off payload.
	n := NewNotifier(NewStore(h.pool), h.pool, h.log, "")
	// Temporarily fan-out using sendPush would hit ALL users for the event.
	// Deliver directly to this user's endpoints instead.
	vapid, err := EnsureVAPID(c.Context(), h.pool)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "vapid_unavailable", err)
	}
	payload := PushPayload{
		Title: "CronCompose",
		Body:  "Browser notifications are working on this device.",
		URL:   "/app/settings",
		Tag:   "push-test",
		Event: "test",
	}
	raw, _ := json.Marshal(payload)
	ok := 0
	for _, s := range items {
		n.deliverPush(c.Context(), vapid, vapidSubscriber(vapid.Subject), s, raw, h.store)
		ok++
	}
	return c.JSON(fiber.Map{"delivered": ok})
}
