package deploys

import (
	"bufio"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

// logs: GET /deploy-runs/:runId/logs[?after=<seq>]. With after, only newer
// chunks are returned; the run page polls this way while live instead of
// downloading the whole log every few seconds.
func (h *handler) logs(c fiber.Ctx) error {
	after := -1
	if raw := c.Query("after"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return jsonError(c, fiber.StatusBadRequest, "bad_after", err)
		}
		after = n
	}
	lines, err := h.store.LogsAfter(c.Context(), c.Params("runId"), after)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": lines})
}

type sseLog struct {
	Stream string `json:"stream"`
	Seq    int    `json:"seq"`
	Chunk  string `json:"chunk"`
}

type sseDone struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Error    string `json:"error"`
}

// stream: GET /deploy-runs/:runId/logs/stream. Server-sent events: the stored
// log first, then live chunks, then one "done" event.
func (h *handler) stream(c fiber.Ctx) error {
	runID := c.Params("runId")
	run, err := h.store.GetRun(c.Context(), runID)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	sub := h.gateway.Broker().Subscribe(runID)
	snapshot, _ := h.store.Logs(c.Context(), runID)
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	done := run.Status != "pending" && run.Status != "running"
	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.gateway.Broker().Unsubscribe(runID, sub)
		// Flush immediately even when the snapshot is empty. Otherwise proxies and
		// EventSource see no body bytes until the first live chunk, and the UI stays
		// on "(no output yet)" for the whole run.
		_, _ = w.WriteString(": connected\n\n")
		_ = w.Flush()
		for _, l := range snapshot {
			writeEvent(w, "log", sseLog{Stream: l.Stream, Seq: l.Seq, Chunk: l.Chunk})
		}
		if done {
			writeDone(w, run.Status, run.ExitCode, run.Error)
			_ = w.Flush()
			return
		}
		_ = w.Flush()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case ev, ok := <-sub:
				if !ok {
					return
				}
				// Write everything already queued, then flush once. One flush per
				// chunk meant one network write per line during a busy build.
				for {
					if ev.Chunk != nil {
						writeEvent(w, "log", sseLog{
							Stream: ev.Chunk.GetStream(),
							Seq:    int(ev.Chunk.GetSeq()),
							Chunk:  string(ev.Chunk.GetData()),
						})
					}
					if ev.Finished != nil {
						code := int(ev.Finished.GetExitCode())
						writeDone(w, ev.Finished.GetStatus(), &code, ev.Finished.GetError())
						_ = w.Flush()
						return
					}
					next, more := nextQueued(sub)
					if !more {
						break
					}
					ev = next
				}
				if err := w.Flush(); err != nil {
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

// nextQueued returns an event that is already waiting, without blocking.
func nextQueued[T any](ch chan T) (T, bool) {
	select {
	case ev, ok := <-ch:
		return ev, ok
	default:
		var zero T
		return zero, false
	}
}

// writeEvent writes one SSE event without flushing. The payload is encoded with
// encoding/json: the old %q formatting produced Go escapes such as \x1b that
// JSON.parse rejects, so those lines vanished from the live view.
func writeEvent(w *bufio.Writer, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.WriteString("event: " + event + "\ndata: ")
	_, _ = w.Write(data)
	_, _ = w.WriteString("\n\n")
}

// writeDone ends an SSE log stream. error must travel with status, otherwise the
// UI flips to "failed" with no message when watching a live run.
func writeDone(w *bufio.Writer, status string, exitCode *int, errMsg string) {
	writeEvent(w, "done", sseDone{Status: status, ExitCode: exitCode, Error: errMsg})
}
