package notify

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

type handler struct {
	log      *slog.Logger
	store    *Store
	audit    audit.Writer
	notifier *Notifier
}

// Register attaches /notification-targets. Admin gating throughout: these rows hold
// SMTP credentials and webhook URLs, and a target is a channel out of the system.
func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool, writer audit.Writer, n *Notifier) {
	h := &handler{log: log, store: NewStore(pool), audit: writer, notifier: n}
	admin := auth.RequireRole("admin")
	r.Get("/notification-targets", admin, h.list)
	r.Post("/notification-targets", admin, h.create)
	r.Patch("/notification-targets/:id", admin, h.patch)
	r.Post("/notification-targets/:id/test", admin, h.test)
	r.Delete("/notification-targets/:id", admin, h.delete)
}

func (h *handler) list(c fiber.Ctx) error {
	items, err := h.store.List(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	out := make([]Target, 0, len(items))
	for _, t := range items {
		out = append(out, t.Redacted())
	}
	return c.JSON(fiber.Map{"items": out})
}

func (h *handler) create(c fiber.Ctx) error {
	var in CreateInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.Kind == "" {
		in.Kind = KindWebhook
	}
	if err := validateCreate(in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields", err)
	}
	t, err := h.store.Insert(c.Context(), in)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "insert_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "notify.target.create", "notification_target",
		t.ID, map[string]any{"name": t.Name, "kind": t.Kind})
	return c.Status(fiber.StatusCreated).JSON(t.Redacted())
}

func (h *handler) patch(c fiber.Ctx) error {
	var in PatchInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	t, err := h.store.Patch(c.Context(), c.Params("id"), in)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "patch_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "notify.target.update", "notification_target",
		t.ID, map[string]any{"name": t.Name, "enabled": t.Enabled})
	return c.JSON(t.Redacted())
}

// test sends a synthetic notification. It reports a delivery failure in the body with
// a 200 rather than as an HTTP error: the request itself succeeded, and what the
// operator needs to read is the SMTP or Slack message, not a status code.
func (h *handler) test(c fiber.Ctx) error {
	t, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "notify.target.test", "notification_target", t.ID, nil)

	if err := h.notifier.Test(c.Context(), t); err != nil {
		return c.JSON(fiber.Map{"delivered": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"delivered": true})
}

func (h *handler) delete(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.store.Delete(c.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return jsonError(c, fiber.StatusNotFound, "not_found", err)
		}
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "notify.target.delete", "notification_target", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// validateCreate checks what each channel actually needs to work. Accepting a target
// that can never deliver only moves the failure to 3am.
func validateCreate(in CreateInput) error {
	if in.Name == "" {
		return errors.New("name is required")
	}
	if !ValidKind(in.Kind) {
		return errors.New("kind must be one of webhook, slack, email")
	}
	switch in.Kind {
	case KindWebhook, KindSlack:
		if in.URL == "" {
			return errors.New("url is required for " + in.Kind + " targets")
		}
	case KindEmail:
		if in.Config["smtp_host"] == "" {
			return errors.New("config.smtp_host is required for email targets")
		}
		if len(splitList(in.Config["to"])) == 0 {
			return errors.New("config.to is required for email targets")
		}
	}
	return nil
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}
