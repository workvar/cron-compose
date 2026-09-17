"use client";

import { useEffect, useState } from "react";
import { clearUpdating, patchUpdating, readUpdating, type UpdatingState } from "@/lib/updating";
import { UpdateProgressMeter } from "@/components/UpdateProgressMeter";
import type { UpdatePhase } from "@/lib/update-progress";
import type { UpdateStatus } from "@/lib/types";

const MAX_MS = 45 * 60 * 1000;
const POLL_MS = 1500;

function asPhase(raw: string | undefined): UpdatePhase | null {
  if (!raw) return null;
  const known: UpdatePhase[] = [
    "offered", "fetching", "cloning", "downloading", "building", "installing",
    "migrating", "stopping", "restarting", "verifying", "done", "failed", "timeout",
  ];
  return known.includes(raw as UpdatePhase) ? (raw as UpdatePhase) : null;
}

async function pollUpdates(): Promise<UpdateStatus | null> {
  try {
    const res = await fetch("/api/updates", { cache: "no-store", signal: AbortSignal.timeout(4000) });
    if (!res.ok) return null;
    return (await res.json()) as UpdateStatus;
  } catch {
    return null;
  }
}

function watchedItems(body: UpdateStatus, state: UpdatingState) {
  const items = body.items ?? [];
  return state.serverIds.length > 0
    ? items.filter((s) => state.serverIds.includes(s.server_id))
    : items.filter((s) => s.stack);
}

function updateSettled(body: UpdateStatus, state: UpdatingState): boolean {
  const want = state.targetVersion.replace(/^v/, "");
  const items = body.items ?? [];
  const watched = watchedItems(body, state);
  if (watched.length === 0) {
    return items.some((s) => (s.current_version || "").replace(/^v/, "") === want);
  }
  return watched.every((s) => {
    const cur = (s.current_version || "").replace(/^v/, "");
    return cur === want || s.update_available === false;
  });
}

function bestProgress(body: UpdateStatus, state: UpdatingState): {
  phase: UpdatePhase;
  detail?: string;
  percent?: number;
} | null {
  const watched = watchedItems(body, state);
  let best: { phase: UpdatePhase; detail?: string; percent: number } | null = null;
  for (const item of watched) {
    const phase = asPhase(item.update_phase);
    if (!phase) continue;
    const percent = item.update_percent ?? 0;
    if (!best || percent >= best.percent) {
      best = { phase, detail: item.update_detail, percent };
    }
  }
  return best;
}

export function UpdatingOverlay() {
  const [state, setState] = useState<UpdatingState | null>(null);
  const [phase, setPhase] = useState<UpdatePhase>("offered");
  const [detail, setDetail] = useState("Sending the update command to the agent");
  const [percent, setPercent] = useState(5);
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    const existing = readUpdating();
    setState(existing);
    if (existing) {
      setPhase(asPhase(existing.phase) ?? "building");
      if (existing.detail) setDetail(existing.detail);
      if (existing.percent) setPercent(existing.percent);
    }
    function onEvt(e: Event) {
      const next = (e as CustomEvent<UpdatingState | null>).detail;
      setState(next);
      if (next) {
        setPhase(asPhase(next.phase) ?? "offered");
        setDetail(next.detail || "Sending the update command to the agent");
        setPercent(next.percent || 5);
        setElapsed(0);
      }
    }
    window.addEventListener("cc-updating", onEvt);
    return () => window.removeEventListener("cc-updating", onEvt);
  }, []);

  useEffect(() => {
    if (!state?.stack) return;

    let cancelled = false;
    let sawDown = false;

    const tick = window.setInterval(() => {
      if (cancelled) return;
      const age = Date.now() - state.startedAt;
      setElapsed(age);
      if (age > MAX_MS) {
        setPhase("timeout");
        setDetail("This is taking longer than expected. Check .run/update.log on the server, then refresh.");
        window.clearInterval(tick);
        return;
      }
      void (async () => {
        const body = await pollUpdates();
        if (!body) {
          const next: UpdatePhase = sawDown ? "restarting" : "stopping";
          const copy = sawDown
            ? "Restarting services — waiting for the control plane to come back"
            : "Stopping the server so it can restart on the new version";
          const pct = sawDown ? 90 : 85;
          sawDown = true;
          setPhase(next);
          setDetail(copy);
          setPercent((p) => (p < pct ? pct : p));
          patchUpdating({ phase: next, detail: copy, percent: pct });
          return;
        }
        const live = bestProgress(body, state);
        if (live && live.phase !== "done") {
          setPhase(live.phase);
          if (live.detail) setDetail(live.detail);
          if (live.percent && live.percent > 0) setPercent(live.percent);
          patchUpdating({ phase: live.phase, detail: live.detail, percent: live.percent });
        }
        if (sawDown || age > 20_000) {
          if (updateSettled(body, state)) {
            setPhase("done");
            setDetail("Update complete. Reloading…");
            setPercent(100);
            window.clearInterval(tick);
            window.setTimeout(() => {
              clearUpdating();
              window.location.reload();
            }, 900);
          } else if (sawDown && !live) {
            setPhase("restarting");
            setDetail("Restarting services — waiting for the control plane to come back");
            setPercent((p) => (p < 90 ? 90 : p));
          }
        }
      })();
    }, POLL_MS);

    return () => {
      cancelled = true;
      window.clearInterval(tick);
    };
  }, [state]);

  if (!state?.stack) return null;

  return (
    <div className="updating-overlay" role="status" aria-live="polite" aria-busy={phase !== "done"}>
      <div className="updating-card">
        <div className="updating-mark" aria-hidden>
          <span className="updating-ring" />
          <span className="updating-ring updating-ring-2" />
          <span className="updating-core">CC</span>
        </div>
        <h1 className="updating-title">CronCompose is updating</h1>
        <UpdateProgressMeter
          stack
          phase={phase}
          detail={detail}
          percent={percent}
          elapsedMs={elapsed}
        />
        {phase === "timeout" && (
          <button type="button" className="button sm" onClick={() => { clearUpdating(); setState(null); }}>
            Dismiss
          </button>
        )}
      </div>
    </div>
  );
}
