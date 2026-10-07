package runs

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
)

type handler struct {
	log    *slog.Logger
	store  *Store
	broker *agentgw.LogBroker
}

func (h *handler) listByJob(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	items, err := h.store.ListByJob(c.Context(), c.Params("id"), limit)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *handler) get(c fiber.Ctx) error {
	r, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(r)
}

func (h *handler) logs(c fiber.Ctx) error {
	lines, err := h.store.Logs(c.Context(), c.Params("id"))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": lines})
}

// stream serves Server-Sent Events for a run: first the already-stored log lines as a
// snapshot, then live chunks from the broker, then a "done" event when the run finishes.
func (h *handler) stream(c fiber.Ctx) error {
	runID := c.Params("id")

	// 404 cleanly before upgrading.
	run, err := h.store.Get(c.Context(), runID)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}

	// Subscribe BEFORE reading the snapshot so we don't miss chunks in between.
	sub := h.broker.Subscribe(runID)
	snapshot, _ := h.store.Logs(c.Context(), runID)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	terminal := run.Status != "pending" && run.Status != "running"

	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.broker.Unsubscribe(runID, sub)

		// Flush immediately even when the snapshot is empty so EventSource/proxies
		// see an open stream before the first live chunk arrives.
		_, _ = w.WriteString(": connected\n\n")
		_ = w.Flush()

		for _, l := range snapshot {
			writeEvent(w, "log",
				fmt.Sprintf(`{"stream":%q,"seq":%d,"chunk":%q}`, l.Stream, l.Seq, l.Chunk))
		}
		if terminal {
			writeEvent(w, "done", fmt.Sprintf(`{"status":%q}`, run.Status))
			return
		}
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case ev, ok := <-sub:
				if !ok {
					return
				}
				if ev.Chunk != nil {
					writeEvent(w, "log",
						fmt.Sprintf(`{"stream":%q,"seq":%d,"chunk":%q}`,
							ev.Chunk.GetStream(), ev.Chunk.GetSeq(), string(ev.Chunk.GetData())))
				}
				if ev.Finished != nil {
					writeEvent(w, "done",
						fmt.Sprintf(`{"status":%q,"exit_code":%d}`,
							ev.Finished.GetStatus(), ev.Finished.GetExitCode()))
					return
				}
			case <-ticker.C:
				_, _ = w.WriteString(": keepalive\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
	return nil
}

func writeEvent(w *bufio.Writer, event, data string) {
	_, _ = w.WriteString("event: " + event + "\ndata: " + data + "\n\n")
	_ = w.Flush()
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}
