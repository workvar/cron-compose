package jobs

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

type handler struct {
	log     *slog.Logger
	store   *Store
	gateway *agentgw.Gateway
	audit   audit.Writer
}

func (h *handler) audited(c fiber.Ctx, action, targetID string, meta map[string]any) {
	h.audit.Write(c.Context(), auth.CurrentUserID(c), action, "job", targetID, meta)
}

func (h *handler) list(c fiber.Ctx) error {
	serverID := c.Query("server")
	rows, err := h.store.List(c.Context(), serverID)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

func (h *handler) create(c fiber.Ctx) error {
	var in CreateInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.Name == "" || in.ScheduleCron == "" || in.ScriptBody == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields",
			errors.New("name, schedule_cron, script_body are required"))
	}
	job, err := h.store.Insert(c.Context(), in)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "insert_failed", err)
	}
	h.fanoutSync(c, job)
	h.audited(c, "job.create", job.ID, map[string]any{"target_kind": job.TargetKind, "name": job.Name})
	return c.Status(fiber.StatusCreated).JSON(job)
}

func (h *handler) get(c fiber.Ctx) error {
	job, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(job)
}

func (h *handler) patch(c fiber.Ctx) error {
	var in PatchInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	before, _ := h.store.Get(c.Context(), c.Params("id"))
	job, err := h.store.Patch(c.Context(), c.Params("id"), in)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "patch_failed", err)
	}
	// Re-sync both old and new targets, so a server that just dropped out of scope
	// also drops the job from its local set on its next SyncJobs.
	h.fanoutSyncBoth(c, before, job)
	h.audited(c, "job.update", job.ID, nil)
	return c.JSON(job)
}

func (h *handler) delete(c fiber.Ctx) error {
	job, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	if err := h.store.Delete(c.Context(), job.ID); err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	h.fanoutSync(c, job)
	h.audited(c, "job.delete", job.ID, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handler) enable(c fiber.Ctx) error  { return h.setEnabled(c, true) }
func (h *handler) disable(c fiber.Ctx) error { return h.setEnabled(c, false) }

func (h *handler) setEnabled(c fiber.Ctx, enabled bool) error {
	id := c.Params("id")
	if err := h.store.SetEnabled(c.Context(), id, enabled); err != nil {
		if errors.Is(err, ErrNotFound) {
			return jsonError(c, fiber.StatusNotFound, "not_found", err)
		}
		return jsonError(c, fiber.StatusInternalServerError, "update_failed", err)
	}
	job, _ := h.store.Get(c.Context(), id)
	h.fanoutSync(c, job)
	action := "job.disable"
	if enabled {
		action = "job.enable"
	}
	h.audited(c, action, job.ID, nil)
	return c.JSON(job)
}

// runNow fans out to every server currently targeted by the job. Returns an array of
// run_ids and a per-server status (queued / agent_offline).
func (h *handler) runNow(c fiber.Ctx) error {
	job, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	serverIDs, err := h.store.MatchingServerIDs(c.Context(), job)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "resolve_failed", err)
	}
	if len(serverIDs) == 0 {
		return jsonError(c, fiber.StatusBadRequest, "no_targets", errors.New("no servers match this job"))
	}
	type result struct {
		ServerID string `json:"server_id"`
		RunID    string `json:"run_id"`
		Status   string `json:"status"` // queued | agent_offline
	}
	results := make([]result, 0, len(serverIDs))
	for _, sid := range serverIDs {
		runID := ids.New()
		status := "queued"
		if err := h.gateway.PushRunNow(sid, job.ID, job.CurrentVersionID, runID); err != nil {
			status = "agent_offline"
		}
		results = append(results, result{ServerID: sid, RunID: runID, Status: status})
	}
	h.audited(c, "job.run", job.ID, map[string]any{"targets": len(results)})
	return c.JSON(fiber.Map{"runs": results})
}

// fanoutSync re-pushes SyncJobs to every server currently matching this job. Offline
// agents are skipped silently (they'll resync on next connect).
func (h *handler) fanoutSync(c fiber.Ctx, job Job) {
	serverIDs, err := h.store.MatchingServerIDs(c.Context(), job)
	if err != nil {
		h.log.Warn("resolve targets for sync failed", "job_id", job.ID, "err", err)
		return
	}
	for _, sid := range serverIDs {
		if err := h.gateway.PushFullSync(c.Context(), sid); err != nil && !errors.Is(err, agentgw.ErrAgentOffline) {
			h.log.Warn("push sync failed", "server_id", sid, "err", err)
		}
	}
}

// fanoutSyncBoth re-syncs both the old and new target sets, so a server that just
// dropped out of scope drops the job from its local cache.
func (h *handler) fanoutSyncBoth(c fiber.Ctx, before, after Job) {
	seen := map[string]struct{}{}
	for _, j := range []Job{before, after} {
		ids, err := h.store.MatchingServerIDs(c.Context(), j)
		if err != nil {
			continue
		}
		for _, sid := range ids {
			if _, ok := seen[sid]; ok {
				continue
			}
			seen[sid] = struct{}{}
			if err := h.gateway.PushFullSync(c.Context(), sid); err != nil && !errors.Is(err, agentgw.ErrAgentOffline) {
				h.log.Warn("push sync failed", "server_id", sid, "err", err)
			}
		}
	}
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}
