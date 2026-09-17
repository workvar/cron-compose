"use client";

import {
  effectivePhase,
  percentForPhase,
  pipelineFor,
  stepState,
  type UpdatePhase,
} from "@/lib/update-progress";

type Props = {
  stack: boolean;
  phase: UpdatePhase;
  detail?: string;
  percent?: number;
  elapsedMs?: number;
};

function clock(ms: number): string {
  const minutes = Math.floor(ms / 60_000);
  const seconds = Math.floor((ms % 60_000) / 1000);
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function UpdateProgressMeter({ stack, phase, detail, percent, elapsedMs = 0 }: Props) {
  const shown = effectivePhase(phase, stack);
  const steps = pipelineFor(stack);
  const pct = percentForPhase(shown, percent);
  const current = steps.find((s) => s.id === shown);
  const status =
    phase === "done"
      ? "Update complete. Reloading…"
      : phase === "timeout"
        ? "This is taking longer than expected."
        : phase === "failed"
          ? detail || "The update failed."
          : detail || current?.hint || current?.label || "Updating…";

  return (
    <div className="update-meter">
      <p className="update-meter-status">{status}</p>
      <div
        className="update-meter-bar"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        aria-label={status}
      >
        <span className="update-meter-fill" style={{ transform: `scaleX(${pct / 100})` }} />
      </div>
      <p className="update-meter-meta mono">
        {pct}%
        {elapsedMs > 0 ? ` · ${clock(elapsedMs)}` : null}
      </p>
      <ol className="update-meter-steps">
        {steps.map((s) => {
          const state = stepState(s.id, shown);
          return (
            <li key={s.id} data-state={state}>
              <span className="update-meter-dot" aria-hidden />
              <span>
                <strong>{s.label}</strong>
                {state === "current" && s.hint && s.hint !== status ? (
                  <span className="update-meter-hint">{s.hint}</span>
                ) : null}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
