"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { IconPlay, IconRestart, IconSkip, IconStop } from "@/components/icons";
import type { LiveStep } from "@/lib/deploy-progress";

type Props = {
  runId: string;
  projectId?: string;
  appName: string;
  /** Selected phase tab. "all" means the active or failed step. */
  phase: string;
  steps: LiveStep[];
  live: boolean;
};

const PHASE_LABEL: Record<string, string> = {
  install: "Install",
  build: "Build",
  cleanup: "Cleanup",
  pre_start: "Pre-start",
  run: "Run",
  start: "Start",
  health: "Health",
};

function stepPhase(phase: string, steps: LiveStep[], prefer: "active" | "failed" | "pending"): string {
  if (phase && phase !== "all") return phase;
  const hit = steps.find((s) => s.state === prefer);
  return hit?.kind || "";
}

/**
 * Start / Skip / Stop / Restart for one app's deployment steps.
 * These never send a process-manager command. Restart re-runs this app's
 * chain only; the other apps stay up.
 */
export function DeployStepActions({ runId, projectId, appName, phase, steps, live }: Props) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const activePhase = stepPhase(phase, steps, "active");
  const startPhase = phase !== "all" ? phase : stepPhase("all", steps, "failed") || stepPhase("all", steps, "pending");
  const ready = Boolean(projectId && appName);
  const phaseLabel = PHASE_LABEL[startPhase] || startPhase;
  const activeLabel = PHASE_LABEL[activePhase] || activePhase;

  async function runChain() {
    if (!projectId) return;
    const ok = window.confirm(
      `Re-run ${appName}'s deploy steps from install through health?\n\n${appName} only. The other apps are left running.`,
    );
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${encodeURIComponent(projectId)}/runs`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ apps: [appName] }),
      });
      const body = (await res.json().catch(() => null)) as
        | { id?: string; error?: { message?: string }; skipped?: string }
        | null;
      if (!res.ok) {
        setError(body?.error?.message || `Could not restart ${appName} (${res.status})`);
        return;
      }
      if (body?.id) {
        router.push(`/deploys/runs/${body.id}`);
        return;
      }
      setError(body?.skipped || "Restart was skipped");
    } catch (e) {
      setError((e as Error).message || `Could not restart ${appName}`);
    } finally {
      setBusy(false);
    }
  }

  async function runStep() {
    if (!projectId || !startPhase) return;
    const ok = window.confirm(`Run the ${phaseLabel} step for ${appName} only?`);
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${encodeURIComponent(projectId)}/runs`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ apps: [appName], only_phase: startPhase }),
      });
      const body = (await res.json().catch(() => null)) as
        | { id?: string; error?: { message?: string }; skipped?: string }
        | null;
      if (!res.ok) {
        setError(body?.error?.message || `Could not start ${phaseLabel} (${res.status})`);
        return;
      }
      if (body?.id) {
        router.push(`/deploys/runs/${body.id}`);
        return;
      }
      setError(body?.skipped || "Step was skipped");
    } catch (e) {
      setError((e as Error).message || `Could not start ${phaseLabel}`);
    } finally {
      setBusy(false);
    }
  }

  async function control(action: "skip" | "stop") {
    const target = activePhase || startPhase;
    if (!target) return;
    const label = PHASE_LABEL[target] || target;
    const ok = window.confirm(
      action === "stop"
        ? `Stop the ${label} step for ${appName}?\n\nOther apps keep going.`
        : `Skip the ${label} step for ${appName}?`,
    );
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploy-runs/${encodeURIComponent(runId)}/steps`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ app: appName, phase: target, action }),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
        setError(body?.error?.message || `Could not ${action} ${label} (${res.status})`);
      }
    } catch (e) {
      setError((e as Error).message || `Could not ${action} ${label}`);
    } finally {
      setBusy(false);
    }
  }

  const canStart = ready && !live && !busy && Boolean(startPhase);
  const canRestart = ready && !live && !busy;
  const canSkip = ready && live && !busy && Boolean(activePhase || startPhase);
  const canStop = ready && live && !busy && Boolean(activePhase);

  return (
    <div className="deploy-proc-actions">
      <div className="deploy-proc-actions-btns" role="group" aria-label={`${appName} deploy steps`}>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={!canStart}
          title={canStart ? `Run the ${phaseLabel} step for ${appName}` : live ? "A deploy is running" : "No step to start"}
          onClick={(e) => {
            e.stopPropagation();
            void runStep();
          }}
        >
          <IconPlay />
          <span>Start</span>
        </button>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={!canSkip}
          title={canSkip ? `Skip the ${activeLabel || phaseLabel} step for ${appName}` : "Skip applies to a running step"}
          onClick={(e) => {
            e.stopPropagation();
            void control("skip");
          }}
        >
          <IconSkip />
          <span>Skip</span>
        </button>
        <button
          type="button"
          className="button danger sm deploy-proc-action"
          disabled={!canStop}
          title={canStop ? `Stop the ${activeLabel} step for ${appName}` : "Stop applies to the running step"}
          onClick={(e) => {
            e.stopPropagation();
            void control("stop");
          }}
        >
          <IconStop />
          <span>Stop</span>
        </button>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={!canRestart}
          title={
            canRestart
              ? `Re-run ${appName} from install through health. Other apps stay up.`
              : live
                ? "Wait until this deploy finishes"
                : "Project is missing"
          }
          onClick={(e) => {
            e.stopPropagation();
            void runChain();
          }}
        >
          <IconRestart />
          <span>{busy ? "Working…" : "Restart"}</span>
        </button>
      </div>
      {error && <p className="form-error" style={{ margin: "6px 0 0" }}>{error}</p>}
    </div>
  );
}
