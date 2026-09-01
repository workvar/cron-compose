package templates

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

type handler struct {
	log   *slog.Logger
	store *Store
	audit audit.Writer
}

// Register attaches /job-templates.
//
// Reading is open to any authenticated role, because a template is just a script
// somebody might run and viewers can already read every job's script. Saving and
// deleting are operator, matching who is allowed to create jobs in the first place.
func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool, writer audit.Writer) {
	h := &handler{log: log, store: NewStore(pool), audit: writer}
	r.Get("/job-templates", h.list)
	r.Get("/job-templates/:id", h.get)
	r.Post("/job-templates", auth.RequireRole("operator"), h.create)
	r.Delete("/job-templates/:id", auth.RequireRole("operator"), h.delete)
}

func (h *handler) list(c fiber.Ctx) error {
	items, err := h.store.List(c.Context(), c.Query("category"))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *handler) get(c fiber.Ctx) error {
	t, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(t)
}

func (h *handler) create(c fiber.Ctx) error {
	var in CreateInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}

	// "Save this job as a template" sends only from_job_id plus whatever the user
	// renamed; the rest is read from the job's current version here.
	if in.FromJobID != "" {
		filled, err := h.store.FromJob(c.Context(), in.FromJobID, in)
		if errors.Is(err, ErrNotFound) {
			return jsonError(c, fiber.StatusNotFound, "job_not_found", err)
		}
		if err != nil {
			return jsonError(c, fiber.StatusInternalServerError, "load_job_failed", err)
		}
		// An explicit name on the request still wins over the job's name.
		if name := trimmed(c); name != "" {
			filled.Name = name
		}
		in = filled
	}

	applyDefaults(&in)
	if in.Name == "" || in.ScriptBody == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields",
			errors.New("name and script_body are required"))
	}

	t, err := h.store.Insert(c.Context(), in, auth.CurrentUserID(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "insert_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "template.create", "job_template", t.ID,
		map[string]any{"name": t.Name, "from_job_id": in.FromJobID})
	return c.Status(fiber.StatusCreated).JSON(t)
}

func (h *handler) delete(c fiber.Ctx) error {
	id := c.Params("id")
	err := h.store.Delete(c.Context(), id)
	switch {
	case errors.Is(err, ErrNotFound):
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	case errors.Is(err, ErrBuiltin):
		return jsonError(c, fiber.StatusForbidden, "builtin", err)
	case err != nil:
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "template.delete", "job_template", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// applyDefaults fills the fields a template can sensibly default, mirroring the job
// defaults so a template and a hand-written job start from the same place.
func applyDefaults(in *CreateInput) {
	if in.Category == "" {
		in.Category = "general"
	}
	if in.Interpreter == "" {
		in.Interpreter = "bash"
	}
	if in.ScheduleCron == "" {
		in.ScheduleCron = "0 * * * *"
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}
}

// trimmed pulls an explicit name out of the raw body without re-binding it, used only
// to let a save-as-template request rename what it copies.
func trimmed(c fiber.Ctx) string {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return ""
	}
	return body.Name
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}
