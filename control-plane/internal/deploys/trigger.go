package deploys

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
)

// A single push can reach this control plane more than once: providers retry webhook
// deliveries, and a repo that has both the push webhook and the generated CI job
// reports the same commit through two different paths within a second or two. Two
// deploys for one commit means two clones racing into the same path, so every trigger
// goes through the checks here first.
const duplicateWindow = 90 * time.Second

// triggerFromWebhook de-duplicates a provider delivery and starts a run. It always
// answers the provider with a success status: a webhook that gets an error back is
// retried, and there is nothing here the provider could fix by retrying.
func (h *handler) triggerFromWebhook(c fiber.Ctx, p Project, branch, deliveryID, commitSHA string) error {
	ctx := c.Context()

	if deliveryID != "" {
		first, err := h.store.ClaimWebhookDelivery(ctx, p.ID, deliveryID, commitSHA)
		if err != nil {
			h.log.Warn("deploy: webhook de-duplication failed", "project_id", p.ID, "err", err)
		} else if !first {
			h.log.Info("deploy: ignoring repeat webhook delivery", "project_id", p.ID, "delivery_id", deliveryID)
			return c.SendStatus(fiber.StatusOK)
		}
	}
	if reason := h.duplicateTrigger(ctx, p, commitSHA); reason != "" {
		h.log.Info("deploy: skipping duplicate webhook trigger", "project_id", p.ID, "reason", reason)
		return c.SendStatus(fiber.StatusOK)
	}
	if _, err := h.startRun(ctx, p, "webhook", branch, ""); err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "run_failed", err)
	}
	return c.SendStatus(fiber.StatusAccepted)
}

// duplicateTrigger returns a human-readable reason to skip this trigger, or "" to go
// ahead. It catches both a second report of the same commit and a run that is still
// in flight for the project, since either would have two deploys writing to the same
// clone path at once.
func (h *handler) duplicateTrigger(ctx context.Context, p Project, commitSHA string) string {
	if commitSHA != "" {
		recent, err := h.store.RecentRunForCommit(ctx, p.ID, commitSHA, duplicateWindow)
		if err != nil {
			h.log.Warn("deploy: duplicate check failed", "project_id", p.ID, "err", err)
		} else if recent {
			return "commit " + shortSHA(commitSHA) + " is already deploying or just deployed"
		}
	}
	active, err := h.store.ActiveRun(ctx, p.ID)
	if err != nil {
		h.log.Warn("deploy: active run check failed", "project_id", p.ID, "err", err)
		return ""
	}
	if active {
		return "a deploy is already running for this project"
	}
	return ""
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
