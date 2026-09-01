package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/metrics"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// dispatch is the single path every mutating connector command takes:
//
//	record the operation -> send to the agent -> record what came back
//
// The row is written BEFORE the send so a command that is never answered still leaves
// evidence that somebody asked for it. Every exit path closes the row out, including
// the ones where we never hear from the agent, which is why the status vocabulary has
// `offline` and `timeout` alongside the agent's own statuses.
func (h *handler) dispatch(c fiber.Ctx, conn Connector, cmd *agentv1.ConnectorCommand,
	auditAction string) (*agentv1.ConnectorResult, string, error) {

	requestID := agentgw.NewRequestID()
	cmd.RequestId = requestID
	actor := auth.CurrentUserID(c)

	opID, err := h.store.StartOperation(c.Context(), Operation{
		ConnectorID: conn.ID,
		ServerID:    conn.ServerID,
		RequestID:   requestID,
		Op:          cmd.GetOp(),
		Action:      cmd.GetAction(),
		Ref:         cmd.GetRef(),
		DryRun:      cmd.GetDryRun(),
		ActorUserID: ptrStr(actor),
	})
	if err != nil {
		return nil, "", err
	}

	// A detached context: the operation row must be closed out even if the browser
	// hangs up mid-apply, and an apply that is already running on the box should be
	// recorded rather than forgotten.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Context()), 60*time.Second)
	defer cancel()

	res, sendErr := h.gateway.SendConnectorCommand(c.Context(), conn.ServerID, cmd)
	switch {
	case errors.Is(sendErr, agentgw.ErrAgentOffline):
		_ = h.store.FinishOperation(finishCtx, opID, "offline", "the agent for this server is not connected", nil)
		metrics.ConnectorOpsTotal.WithLabelValues(cmd.GetOp(), "offline").Inc()
		return nil, opID, sendErr
	case errors.Is(sendErr, agentgw.ErrCommandTimeout):
		_ = h.store.FinishOperation(finishCtx, opID, "timeout",
			"the agent did not answer; the command may still have taken effect", nil)
		metrics.ConnectorOpsTotal.WithLabelValues(cmd.GetOp(), "timeout").Inc()
		return nil, opID, sendErr
	case sendErr != nil:
		_ = h.store.FinishOperation(finishCtx, opID, "failed", sendErr.Error(), nil)
		metrics.ConnectorOpsTotal.WithLabelValues(cmd.GetOp(), "failed").Inc()
		return nil, opID, sendErr
	}

	steps := stepsFromProto(res.GetSteps())
	_ = h.store.FinishOperation(finishCtx, opID, res.GetStatus(), res.GetMessage(), steps)
	metrics.ConnectorOpsTotal.WithLabelValues(cmd.GetOp(), res.GetStatus()).Inc()

	h.audited(c, auditAction, conn.ID, map[string]any{
		"server_id": conn.ServerID,
		"kind":      conn.Kind,
		"op":        cmd.GetOp(),
		"action":    cmd.GetAction(),
		"ref":       cmd.GetRef(),
		"dry_run":   cmd.GetDryRun(),
		"status":    res.GetStatus(),
	})
	return res, opID, nil
}

// action: POST /connectors/:id/actions. Operator and above.
func (h *handler) action(c fiber.Ctx) error {
	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}

	var req LifecycleRequest
	if err := c.Bind().Body(&req); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "invalid_body", err)
	}
	if req.Action == "" {
		return jsonError(c, fiber.StatusBadRequest, "invalid_body", errors.New("action is required"))
	}
	if !conn.Capabilities["can_lifecycle"] {
		return jsonError(c, fiber.StatusConflict, "not_manageable",
			errors.New("this connector reported that the agent cannot drive it"))
	}

	res, opID, err := h.dispatch(c, conn, &agentv1.ConnectorCommand{
		Op:            "lifecycle",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
		Ref:           req.Ref,
		Action:        req.Action,
	}, "connector.action")
	if err != nil {
		return h.dispatchError(c, err, opID)
	}
	return c.JSON(fiber.Map{
		"operation_id": opID,
		"status":       res.GetStatus(),
		"message":      res.GetMessage(),
		"steps":        stepsFromProto(res.GetSteps()),
	})
}

// readConfig: GET /connectors/:id/config?path=... Admin only.
//
// Reading a config file is admin-gated rather than viewer-gated because config files
// routinely contain upstream hostnames, internal ports, and occasionally credentials.
func (h *handler) readConfig(c fiber.Ctx) error {
	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	path := c.Query("path")
	if path == "" {
		return jsonError(c, fiber.StatusBadRequest, "invalid_query", errors.New("path is required"))
	}

	res, opID, err := h.dispatch(c, conn, &agentv1.ConnectorCommand{
		Op:            "read",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
		Ref:           path,
	}, "connector.config.read")
	if err != nil {
		return h.dispatchError(c, err, opID)
	}
	if res.GetStatus() != "succeeded" {
		return jsonError(c, statusToHTTP(res.GetStatus()), res.GetStatus(), errors.New(res.GetMessage()))
	}
	return c.JSON(ConfigReadResponse{
		Path:     path,
		Content:  string(res.GetContent()),
		Checksum: res.GetChecksum(),
	})
}

// writeConfig: POST /connectors/:id/config. Admin only.
//
// The current bytes are snapshotted here, in the control plane, before the command
// goes out. The agent keeps its own backup for the in-flight rollback, but that one
// lives only as long as the request; this one is what lets an operator go back to
// last Tuesday's config.
func (h *handler) writeConfig(c fiber.Ctx) error {
	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}

	var req ConfigWriteRequest
	if err := c.Bind().Body(&req); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "invalid_body", err)
	}
	if req.Path == "" {
		return jsonError(c, fiber.StatusBadRequest, "invalid_body", errors.New("path is required"))
	}
	if !conn.Capabilities["can_edit"] && !req.DryRun {
		return jsonError(c, fiber.StatusConflict, "not_editable",
			errors.New("this connector reported that the agent cannot edit its config"))
	}

	if !req.DryRun {
		h.snapshotBefore(c, conn, req.Path)
	}

	res, opID, err := h.dispatch(c, conn, &agentv1.ConnectorCommand{
		Op:            "apply",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
		Ref:           req.Path,
		Content:       []byte(req.Content),
		BaseChecksum:  req.BaseChecksum,
		DryRun:        req.DryRun,
	}, "connector.config.apply")
	if err != nil {
		return h.dispatchError(c, err, opID)
	}
	return c.JSON(fiber.Map{
		"operation_id": opID,
		"status":       res.GetStatus(),
		"message":      res.GetMessage(),
		"checksum":     res.GetChecksum(),
		"steps":        stepsFromProto(res.GetSteps()),
	})
}

// rollback: POST /connectors/:id/snapshots/:snapshotID/restore. Admin only.
func (h *handler) rollback(c fiber.Ctx) error {
	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}

	snap, content, err := h.store.GetSnapshot(c.Context(), c.Params("snapshotID"))
	if errors.Is(err, ErrSnapshotNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	if snap.ConnectorID != conn.ID {
		return jsonError(c, fiber.StatusBadRequest, "mismatched_snapshot",
			errors.New("that snapshot belongs to a different connector"))
	}

	// Snapshot the current state too: rolling back is itself a change worth undoing.
	h.snapshotBefore(c, conn, snap.Ref)

	res, opID, err := h.dispatch(c, conn, &agentv1.ConnectorCommand{
		Op:            "rollback",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
		Ref:           snap.Ref,
		Content:       content,
	}, "connector.config.rollback")
	if err != nil {
		return h.dispatchError(c, err, opID)
	}
	return c.JSON(fiber.Map{
		"operation_id": opID,
		"status":       res.GetStatus(),
		"message":      res.GetMessage(),
		"steps":        stepsFromProto(res.GetSteps()),
	})
}

// listOperations: GET /connectors/:id/operations. Any authenticated role.
func (h *handler) listOperations(c fiber.Ctx) error {
	rows, err := h.store.ListOperations(c.Context(), c.Params("id"), queryLimit(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

// listPorts: GET /connectors/:id/ports. Any authenticated role. Live from the agent,
// not the discovery cache, because listen sockets change independently of the
// five-minute sweep. Not recorded as an operation: refreshing the tab would drown
// the history in no-op reads.
func (h *handler) listPorts(c fiber.Ctx) error {
	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}

	res, err := h.gateway.SendConnectorCommand(c.Context(), conn.ServerID, &agentv1.ConnectorCommand{
		RequestId:     agentgw.NewRequestID(),
		Op:            "ports",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
	})
	if err != nil {
		return h.dispatchError(c, err, "")
	}
	if res.GetStatus() != "succeeded" {
		return jsonError(c, statusToHTTP(res.GetStatus()), res.GetStatus(), errors.New(res.GetMessage()))
	}
	items := []PortRow{}
	if b := res.GetPayloadJson(); len(b) > 0 {
		if err := json.Unmarshal(b, &items); err != nil {
			return jsonError(c, fiber.StatusBadGateway, "bad_payload", err)
		}
	}
	return c.JSON(fiber.Map{"items": items})
}

// listSnapshots: GET /connectors/:id/snapshots. Admin only (they hold config bytes).
func (h *handler) listSnapshots(c fiber.Ctx) error {
	rows, err := h.store.ListSnapshots(c.Context(), c.Params("id"), c.Query("path"), queryLimit(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": rows})
}

// snapshotBefore reads the file through the agent and stores the bytes. Best effort:
// a connector that cannot read the file (it may not exist yet) must not block the
// write, so failures are logged and swallowed.
func (h *handler) snapshotBefore(c fiber.Ctx, conn Connector, path string) {
	res, err := h.gateway.SendConnectorCommand(c.Context(), conn.ServerID, &agentv1.ConnectorCommand{
		RequestId:     agentgw.NewRequestID(),
		Op:            "read",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
		Ref:           path,
	})
	if err != nil || res.GetStatus() != "succeeded" {
		h.log.Info("connector snapshot skipped", "connector_id", conn.ID, "path", path,
			"err", err, "status", res.GetStatus())
		return
	}
	sum := res.GetChecksum()
	if sum == "" {
		raw := sha256.Sum256(res.GetContent())
		sum = hex.EncodeToString(raw[:])
	}
	actor := auth.CurrentUserID(c)
	if _, err := h.store.SaveSnapshot(c.Context(), Snapshot{
		ConnectorID: conn.ID,
		Ref:         path,
		Checksum:    sum,
		Reason:      "pre_apply",
		ActorUserID: ptrStr(actor),
	}, res.GetContent()); err != nil {
		h.log.Warn("connector snapshot save failed", "connector_id", conn.ID, "err", err)
		return
	}
	if err := h.store.PruneSnapshots(c.Context(), conn.ID, path, 20); err != nil {
		h.log.Warn("connector snapshot prune failed", "connector_id", conn.ID, "err", err)
	}
}

// dispatchError turns a transport failure into the right HTTP status. The operation id
// is always returned so the UI can link to the recorded attempt.
func (h *handler) dispatchError(c fiber.Ctx, err error, opID string) error {
	status := fiber.StatusInternalServerError
	code := "command_failed"
	switch {
	case errors.Is(err, agentgw.ErrAgentOffline):
		status, code = fiber.StatusServiceUnavailable, "agent_offline"
	case errors.Is(err, agentgw.ErrCommandTimeout):
		status, code = fiber.StatusGatewayTimeout, "agent_timeout"
	}
	return c.Status(status).JSON(fiber.Map{
		"error":        fiber.Map{"code": code, "message": err.Error()},
		"operation_id": opID,
	})
}

// statusToHTTP maps an agent-reported status onto an HTTP code for the read path.
func statusToHTTP(status string) int {
	switch status {
	case "invalid":
		return fiber.StatusUnprocessableEntity
	case "unauthorized":
		return fiber.StatusForbidden
	case "unsupported":
		return fiber.StatusNotImplemented
	}
	return fiber.StatusBadGateway
}

func stepsFromProto(in []*agentv1.ConnectorStep) []Step {
	out := make([]Step, 0, len(in))
	for _, s := range in {
		out = append(out, Step{
			Name:     s.GetName(),
			OK:       s.GetOk(),
			Output:   s.GetOutput(),
			ExitCode: s.GetExitCode(),
		})
	}
	return out
}

// queryLimit reads ?limit=, matching the runs endpoints. The store clamps the value,
// so an absurd number here is harmless.
func queryLimit(c fiber.Ctx) int {
	n, _ := strconv.Atoi(c.Query("limit", "50"))
	return n
}
