"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { LogLine } from "@/lib/types";

/** How long live SSE lines are held before one React update. Rendering the run
 *  board re-parses the whole log, so one update per line froze the tab during a
 *  busy build and the terminal fell behind the server. */
const FLUSH_MS = 80;
/** Safety-net poll while the run is live. It only asks for newer rows. */
const POLL_MS = 4000;
/** Rows from parallel apps can be stored slightly out of seq order, so each
 *  poll re-asks for a small window below its cursor. Duplicates are dropped. */
const POLL_OVERLAP = 50;

export type DeployDone = {
  status: string;
  exit_code?: number;
  error?: string;
};

const keyOf = (l: LogLine) => `${l.stream}:${l.seq}`;

/** Drops lines already held and records the rest as seen. Kept outside the
 *  state updater so React's double-invoked updaters cannot lose lines. */
export function takeNewLines(next: LogLine[], seen: Set<string>): LogLine[] {
  const extra: LogLine[] = [];
  for (const l of next) {
    const k = keyOf(l);
    if (seen.has(k)) continue;
    seen.add(k);
    extra.push(l);
  }
  return extra.sort((a, b) => a.seq - b.seq);
}

/** Inserts sorted new lines into a seq-sorted list. Appends when the batch is
 *  newer than everything held, which is the common live case. */
export function insertLogLines(prev: LogLine[], extra: LogLine[]): LogLine[] {
  if (extra.length === 0) return prev;
  const last = prev.length > 0 ? prev[prev.length - 1].seq : -Infinity;
  if (extra[0].seq >= last) return prev.concat(extra);
  return prev.concat(extra).sort((a, b) => a.seq - b.seq);
}

/**
 * Live deploy log for one run: SSE for low latency, batched into one state
 * update per FLUSH_MS, plus an incremental poll that fills anything the live
 * stream missed. onDone fires once when the stream reports the run finished.
 */
export function useDeployRunLogs(runId: string, live: boolean, onDone: (d: DeployDone) => void) {
  const [logs, setLogs] = useState<LogLine[]>([]);
  const seen = useRef(new Set<string>());
  const queue = useRef<LogLine[]>([]);
  const timer = useRef<number | null>(null);
  const cursor = useRef(-1);
  const doneRef = useRef(onDone);
  useEffect(() => {
    doneRef.current = onDone;
  }, [onDone]);

  const push = useCallback((lines: LogLine[]) => {
    const extra = takeNewLines(lines, seen.current);
    if (extra.length === 0) return;
    setLogs((prev) => insertLogLines(prev, extra));
  }, []);

  const flushQueue = useCallback(() => {
    timer.current = null;
    const lines = queue.current;
    queue.current = [];
    push(lines);
  }, [push]);

  // Reset when switching runs.
  useEffect(() => {
    seen.current = new Set();
    queue.current = [];
    cursor.current = -1;
    setLogs([]);
  }, [runId]);

  // Incremental snapshot: works even when EventSource fails (proxy buffering, blip).
  useEffect(() => {
    let cancelled = false;
    const load = () => {
      const after = cursor.current >= 0 ? `?after=${Math.max(-1, cursor.current - POLL_OVERLAP)}` : "";
      fetch(`/api/deploy-runs/${runId}/logs${after}`)
        .then((r) => r.json() as Promise<{ items: LogLine[] }>)
        .then((d) => {
          if (cancelled || !Array.isArray(d.items)) return;
          for (const l of d.items) if (l.seq > cursor.current) cursor.current = l.seq;
          push(d.items);
        })
        .catch(() => { /* ignore */ });
    };
    load();
    if (!live) return () => { cancelled = true; };
    const id = window.setInterval(load, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [runId, live, push]);

  useEffect(() => {
    const es = new EventSource(`/api/deploy-runs/${runId}/logs/stream`);
    es.addEventListener("log", (ev) => {
      try {
        queue.current.push(JSON.parse((ev as MessageEvent).data) as LogLine);
        if (timer.current === null) timer.current = window.setTimeout(flushQueue, FLUSH_MS);
      } catch { /* ignore */ }
    });
    es.addEventListener("done", (ev) => {
      if (timer.current !== null) window.clearTimeout(timer.current);
      flushQueue();
      try {
        doneRef.current(JSON.parse((ev as MessageEvent).data) as DeployDone);
      } catch {
        doneRef.current({ status: "" });
      }
      es.close();
    });
    // Do not close on the first error: EventSource reconnects by default unless we
    // close it. Closing here left the page stuck on "(no output yet)".
    es.onerror = () => { /* allow browser reconnect */ };
    return () => {
      es.close();
      if (timer.current !== null) window.clearTimeout(timer.current);
      timer.current = null;
    };
  }, [runId, flushQueue]);

  return logs;
}
