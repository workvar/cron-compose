"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { beginAgentUpdating, clearAgentUpdating, patchAgentUpdating, readAgentUpdating } from "@/lib/agent-update";
import type { UpdateStatus } from "@/lib/types";
import type { UpdatePhase } from "@/lib/update-progress";

export type AgentUpdatePhase = "idle" | UpdatePhase;

const POLL_MS = 1500;
const MAX_MS = 20 * 60 * 1000; // give up narrating after 20 minutes; agent may still finish
const DOWN_GRACE_MS = 8000; // don't call it "settled" off a single early poll

function asPhase(raw: string | undefined): UpdatePhase | null {
  if (!raw) return null;
  const known: UpdatePhase[] = [
    "offered", "fetching", "cloning", "downloading", "building", "installing",
    "migrating", "stopping", "restarting", "verifying", "done", "failed", "timeout",
  ];
  return known.includes(raw as UpdatePhase) ? (raw as UpdatePhase) : null;
}

/**
 * Drives the visible progress of a single agent's self-rebuild, and makes it
 * survive a page refresh: the moment update() is confirmed we record it in
 * sessionStorage, and on mount we check for an already-running update so a
 * reload never shows a fresh "Update" button while the agent is mid-rebuild.
 *
 * Live stage text and percent come from GET /updates (the agent streams
 * UpdateProgress). Offline / timeout are inferred only when the agent is
 * silent: going offline after we saw a start means it is stopping to restart.
 */
export function useAgentUpdateProgress(serverId: string, targetVersion: string | undefined, active: boolean) {
  const router = useRouter();
  const [phase, setPhase] = useState<AgentUpdatePhase>("idle");
  const [detail, setDetail] = useState<string>("");
  const [percent, setPercent] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const startedAtRef = useRef<number | null>(null);
  const sawDownRef = useRef(false);

  function applyLive(next: UpdatePhase, nextDetail?: string, nextPercent?: number) {
    setPhase(next);
    if (nextDetail) setDetail(nextDetail);
    if (typeof nextPercent === "number" && nextPercent > 0) setPercent(nextPercent);
    patchAgentUpdating(serverId, { phase: next, detail: nextDetail, percent: nextPercent });
  }

  // Pick up an update already in flight (e.g. after a browser refresh), or
  // clean up a stale entry once the server confirms the update landed.
  useEffect(() => {
    const existing = readAgentUpdating(serverId);
    if (!existing) return;
    if (!active) {
      clearAgentUpdating(serverId);
      return;
    }
    const age = Date.now() - existing.startedAt;
    startedAtRef.current = existing.startedAt;
    setElapsed(age);
    const restored = asPhase(existing.phase) ?? "building";
    setPhase(age > MAX_MS ? "timeout" : restored);
    if (existing.detail) setDetail(existing.detail);
    if (existing.percent) setPercent(existing.percent);
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
        applyLive("timeout", "This is taking longer than expected. Check the agent's update log on the server.");
        return;
      }
      void (async () => {
        try {
          const res = await fetch("/api/updates", { cache: "no-store", signal: AbortSignal.timeout(4000) });
          if (!res.ok) return;
          const body = (await res.json()) as UpdateStatus;
          const item = body.items.find((s) => s.server_id === serverId);
          if (!item) return;
          const live = asPhase(item.update_phase);
          if (live && live !== "done" && !cancelled) {
            applyLive(live, item.update_detail, item.update_percent);
          }
          if (item.status === "offline") {
            sawDownRef.current = true;
            if (!cancelled && live !== "done" && live !== "failed") {
              applyLive(
                live === "stopping" ? "stopping" : "restarting",
                item.update_detail || "Stopping the agent so it can restart on the new version",
                item.update_percent || 90,
              );
            }
            return;
          }
          const want = (targetVersion ?? "").replace(/^v/, "");
          const cur = (item.current_version || "").replace(/^v/, "");
          const settled = (want.length > 0 && cur === want) || item.update_available === false;
          if (settled && (sawDownRef.current || age > DOWN_GRACE_MS)) {
            if (cancelled) return;
            applyLive("done", "Update complete — refreshing…", 100);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, serverId, targetVersion, router]);

  function start() {
    if (!targetVersion) return;
    beginAgentUpdating(serverId, targetVersion);
    startedAtRef.current = Date.now();
    sawDownRef.current = false;
    setElapsed(0);
    setPercent(5);
    setDetail("Sending the update command to the agent");
    setPhase("offered");
  }

  function dismiss() {
    clearAgentUpdating(serverId);
    startedAtRef.current = null;
    setPhase("idle");
    setDetail("");
    setPercent(0);
  }

  return { phase, detail, percent, elapsed, start, dismiss };
}
