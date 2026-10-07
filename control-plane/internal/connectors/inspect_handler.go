package connectors

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

type inspectInput struct {
	ChallengeID string          `json:"challenge_id"`
	Credential  json.RawMessage `json:"credential"`
}

// inspectObject: POST /connectors/:id/objects/:ref/inspect
// Requires passkey step-up. Returns full process detail including env from the agent.
func (h *handler) inspectObject(c fiber.Ctx) error {
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
			errors.New("enroll a passkey before revealing process environment"))
	}
	var in inspectInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.ChallengeID == "" {
		in.ChallengeID = c.Cookies(auth.ChallengeCookie)
	}
	if err := h.stepUp.VerifyStepUp(c.Context(), auth.CurrentUserID(c), in.ChallengeID, in.Credential); err != nil {
		return jsonError(c, fiber.StatusForbidden, "invalid_assertion", err)
	}

	conn, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	ref := c.Params("ref")
	if ref == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_ref", errors.New("ref is required"))
	}

	res, err := h.gateway.SendConnectorCommand(c.Context(), conn.ServerID, &agentv1.ConnectorCommand{
		RequestId:     agentgw.NewRequestID(),
		Op:            "inspect",
		ConnectorKind: conn.Kind,
		ConnectorId:   conn.Instance,
		Ref:           ref,
	})
	if err != nil {
		return h.dispatchError(c, err, "")
	}
	if res.GetStatus() != "succeeded" {
		return jsonError(c, statusToHTTP(res.GetStatus()), res.GetStatus(), errors.New(res.GetMessage()))
	}

	var detail map[string]any
	body := res.GetContent()
	if len(body) == 0 {
		body = res.GetPayloadJson()
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &detail); err != nil {
			return jsonError(c, fiber.StatusBadGateway, "bad_payload", err)
		}
	}
	h.audited(c, "connector.inspect", conn.ID, map[string]any{
		"kind": conn.Kind, "ref": ref,
	})
	return c.JSON(fiber.Map{"detail": detail})
}

// deployInventory: GET /servers/:id/deploy-inventory
// Groups pm2/systemd/docker connectors and their object resources for the Deploy UI.
func (h *handler) deployInventory(c fiber.Ctx) error {
	serverID := c.Params("id")
	conns, err := h.store.ListByServer(c.Context(), serverID)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	kinds := map[string]bool{"pm2": true, "systemd": true, "docker": true}
	type kindBucket struct {
		Kind       string     `json:"kind"`
		Connectors []Connector `json:"connectors"`
		Objects    []Resource `json:"objects"`
		Count      int        `json:"count"`
	}
	order := []string{"pm2", "systemd", "docker"}
	buckets := map[string]*kindBucket{}
	for _, k := range order {
		buckets[k] = &kindBucket{Kind: k, Connectors: []Connector{}, Objects: []Resource{}}
	}
	for _, conn := range conns {
		if !kinds[conn.Kind] {
			continue
		}
		b := buckets[conn.Kind]
		b.Connectors = append(b.Connectors, conn)
		b.Count += conn.ObjectCount
		resources, err := h.store.ListResources(c.Context(), conn.ID)
		if err != nil {
			return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
		}
		for _, r := range resources {
			if r.Type != "object" {
				continue
			}
			// Stamp connector id so the UI can call inspect/import.
			if r.Attributes == nil {
				r.Attributes = map[string]string{}
			}
			r.Attributes["connector_id"] = conn.ID
			b.Objects = append(b.Objects, r)
		}
	}
	items := make([]kindBucket, 0, len(order))
	total := 0
	for _, k := range order {
		b := buckets[k]
		total += b.Count
		items = append(items, *b)
	}
	return c.JSON(fiber.Map{"server_id": serverID, "total": total, "items": items})
}
