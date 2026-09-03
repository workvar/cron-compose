"use client";

import { useEffect, useState } from "react";
import { clearUpdating, readUpdating, type UpdatingState } from "@/lib/updating";

const MAX_MS = 45 * 60 * 1000;
const POLL_MS = 3000;

async function updatesReachable(): Promise<boolean> {
  try {
    const res = await fetch("/api/updates", { cache: "no-store", signal: AbortSignal.timeout(4000) });
    return res.ok;
  } catch {
    return false;
  }
}

async function updateSettled(state: UpdatingState): Promise<boolean> {
  try {
    const res = await fetch("/api/updates", { cache: "no-store", signal: AbortSignal.timeout(4000) });
    if (!res.ok) return false;
    const body = (await res.json()) as {
      latest_version?: string;
      items?: {
        server_id: string;
        stack?: boolean;
        current_version?: string;
        update_available?: boolean;
      }[];
    };
    const want = state.targetVersion.replace(/^v/, "");
    const items = body.items ?? [];
    const watched =
      state.serverIds.length > 0
        ? items.filter((s) => state.serverIds.includes(s.server_id))
        : items.filter((s) => s.stack);
    if (watched.length === 0) {
      // Fall back: any host already on the target.
      return items.some((s) => (s.current_version || "").replace(/^v/, "") === want);
    }
    return watched.every((s) => {
      const cur = (s.current_version || "").replace(/^v/, "");
      return cur === want || s.update_available === false;
    });
  } catch {
    return false;
  }
}

export function UpdatingOverlay() {
  const [state, setState] = useState<UpdatingState | null>(null);
  const [phase, setPhase] = useState<"building" | "restarting" | "done" | "timeout">("building");
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    setState(readUpdating());
    function onEvt(e: Event) {
      const detail = (e as CustomEvent<UpdatingState | null>).detail;
      setState(detail);
      if (detail) {
        setPhase("building");
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
        window.clearInterval(tick);
        return;
      }
      void (async () => {
        const up = await updatesReachable();
        if (!up) {
          sawDown = true;
          setPhase("restarting");
          return;
        }
        if (sawDown || age > 20_000) {
          if (await updateSettled(state)) {
            setPhase("done");
            window.clearInterval(tick);
            window.setTimeout(() => {
              clearUpdating();
              window.location.reload();
            }, 900);
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

  const minutes = Math.floor(elapsed / 60_000);
  const seconds = Math.floor((elapsed % 60_000) / 1000);
  const clock = `${minutes}:${seconds.toString().padStart(2, "0")}`;

  const copy =
    phase === "done"
      ? "Update complete. Reloading…"
      : phase === "timeout"
        ? "This is taking longer than expected. Check .run/update.log on the server, then refresh."
        : phase === "restarting"
          ? "Restarting services…"
          : `Building ${state.targetVersion} from source…`;

  return (
    <div className="updating-overlay" role="status" aria-live="polite" aria-busy={phase !== "done"}>
      <div className="updating-card">
        <div className="updating-mark" aria-hidden>
          <span className="updating-ring" />
          <span className="updating-ring updating-ring-2" />
          <span className="updating-core">CC</span>
        </div>
        <h1 className="updating-title">CronCompose is updating</h1>
        <p className="updating-copy">{copy}</p>
        <p className="updating-meta mono">{clock}</p>
        {phase === "timeout" && (
          <button type="button" className="button sm" onClick={() => { clearUpdating(); setState(null); }}>
            Dismiss
          </button>
        )}
      </div>
    </div>
  );
}
