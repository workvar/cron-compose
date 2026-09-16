"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { beginAgentUpdating, clearAgentUpdating, readAgentUpdating } from "@/lib/agent-update";
import type { UpdateStatus } from "@/lib/types";

export type AgentUpdatePhase = "idle" | "building" | "restarting" | "done" | "timeout";

const POLL_MS = 3000;
const MAX_MS = 20 * 60 * 1000; // give up narrating after 20 minutes; agent may still finish
const DOWN_GRACE_MS = 8000; // don't call it "settled" off a single early poll

/**
 * Drives the visible progress of a single agent's self-rebuild, and makes it
 * survive a page refresh: the moment update() is confirmed we record it in
 * sessionStorage, and on mount we check for an already-running update so a
 * reload never shows a fresh "Update" button while the agent is mid-rebuild.
 */
export function useAgentUpdateProgress(serverId: string, targetVersion: string | undefined, active: boolean) {
  const router = useRouter();
  const [phase, setPhase] = useState<AgentUpdatePhase>("idle");
  const [elapsed, setElapsed] = useState(0);
  const startedAtRef = useRef<number | null>(null);
  const sawDownRef = useRef(false);

  // Pick up an update already in flight (e.g. after a browser refresh), or
  // clean up a stale entry once the server confirms the update landed.
  useEffect(() => {
    const existing = readAgentUpdating(serverId);
    if (!existing) return;
    if (!active) {
      // Parent already reports no update pending — the previous run finished
      // (or was superseded) since we last polled it.
      clearAgentUpdating(serverId);
      return;
    }
    const age = Date.now() - existing.startedAt;
    startedAtRef.current = existing.startedAt;
    setElapsed(age);
    setPhase(age > MAX_MS ? "timeout" : "building");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverId]);

  useEffect(() => {
    if (phase === "idle" || phase === "done" || phase === "timeout") return;
    let cancelled = false;

    const tick = window.setInterval(() => {
      if (cancelled || startedAtRef.current === null) return;
      const age = Date.now() - startedAtRef.current;
      setElapsed(age);
      if (age > MAX_MS) {
        setPhase("timeout");
        return;
      }
      void (async () => {
        try {
          const res = await fetch("/api/updates", { cache: "no-store", signal: AbortSignal.timeout(4000) });
          if (!res.ok) return;
          const body = (await res.json()) as UpdateStatus;
          const item = body.items.find((s) => s.server_id === serverId);
          if (!item) return;
          if (item.status === "offline") {
            sawDownRef.current = true;
            if (!cancelled) setPhase("restarting");
            return;
          }
          const want = (targetVersion ?? "").replace(/^v/, "");
          const cur = (item.current_version || "").replace(/^v/, "");
          const settled = (want.length > 0 && cur === want) || item.update_available === false;
          if (settled && (sawDownRef.current || age > DOWN_GRACE_MS)) {
            if (cancelled) return;
            setPhase("done");
            clearAgentUpdating(serverId);
            window.setTimeout(() => router.refresh(), 600);
          }
        } catch {
          // Transient network hiccup — keep polling, don't flip phase on it.
        }
      })();
    }, POLL_MS);

    return () => {
      cancelled = true;
      window.clearInterval(tick);
    };
  }, [phase, serverId, targetVersion, router]);

  function start() {
    if (!targetVersion) return;
    beginAgentUpdating(serverId, targetVersion);
    startedAtRef.current = Date.now();
    sawDownRef.current = false;
    setElapsed(0);
    setPhase("building");
  }

  function dismiss() {
    clearAgentUpdating(serverId);
    startedAtRef.current = null;
    setPhase("idle");
  }

  return { phase, elapsed, start, dismiss };
}
