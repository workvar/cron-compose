"use client";

import { useEffect, useState } from "react";
import { IconPause, IconPlay, IconRestart, IconStop } from "@/components/icons";
import { useConnectorCommand } from "@/components/connectors/useConnectorCommand";
import { StepList } from "@/components/connectors/StepList";
import { isProcessPaused, isProcessRunning, processActionRef, type ProcessRow } from "@/lib/project-processes";

type Props = {
  row: ProcessRow | null;
  /** Soft-disable while deploy is still writing the process list. */
  enabled?: boolean;
  onChanged?: () => void;
};

/**
 * Start / Pause / Stop / Restart for one process card.
 * All four stay visible; Pause/Resume and Start enable based on live state.
 */
export function ProcessTaskActions({ row, enabled = true, onChanged }: Props) {
  const { busy, error, result, send } = useConnectorCommand();
  const [forced, setForced] = useState<"running" | "stopped" | "paused" | null>(null);
  const reported = row?.resource?.state;

  useEffect(() => {
    setForced(null);
  }, [reported]);

  if (!row || row.pm === "none") return null;

  const resource = row.resource;
  const ref = processActionRef(row);
  const paused = forced === "paused" || (forced === null && isProcessPaused(resource?.state));
  const running = forced === "running" || (forced === null && isProcessRunning(resource?.state));
  const ready = Boolean(enabled && row.connectorId && ref);
  const label = row.label || row.processName;
  const blocked = !enabled
    ? "Wait for this deploy step to finish"
    : !row.connectorId
      ? `No ${row.pm} connector on this server yet — open Connectors`
      : !ref
        ? "Process name is missing"
        : "";

  async function run(action: string) {
    if (!row?.connectorId || !ref) return;
    if (action === "stop") {
      const ok = window.confirm(`Stop ${label}?`);
      if (!ok) return;
    }
    const body = await send(`/api/connectors/${encodeURIComponent(row.connectorId)}/actions`, {
      method: "POST",
      body: JSON.stringify({ action, ref }),
    });
    if (body?.status === "succeeded") {
      if (action === "pause") setForced("paused");
      else if (action === "stop") setForced("stopped");
      else if (action === "unpause" || action === "restart" || action === "start") setForced("running");
      onChanged?.();
    }
  }

  const canStart = ready && (!running || paused);
  const canPause = ready && running && !paused;
  const canStop = ready;
  const canRestart = ready;

  return (
    <div className="deploy-proc-actions">
      <div className="deploy-proc-actions-btns" role="group" aria-label={`${label} process controls`}>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={busy || !canStart}
          title={
            blocked ||
            (paused ? `Resume ${label}` : running ? `${label} is already running` : `Start ${label}`)
          }
          onClick={() => void run(paused ? "unpause" : "start")}
        >
          <IconPlay />
          <span>{paused ? "Resume" : "Start"}</span>
        </button>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={busy || !canPause}
          title={blocked || (running ? `Pause ${label}` : `${label} is not running`)}
          onClick={() => void run("pause")}
        >
          <IconPause />
          <span>Pause</span>
        </button>
        <button
          type="button"
          className="button danger sm deploy-proc-action"
          disabled={busy || !canStop}
          title={blocked || `Stop ${label}`}
          onClick={() => void run("stop")}
        >
          <IconStop />
          <span>Stop</span>
        </button>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={busy || !canRestart}
          title={blocked || `Restart ${label}`}
          onClick={() => void run("restart")}
        >
          <IconRestart />
          <span>Restart</span>
        </button>
      </div>
      {!ready && blocked && (
        <p className="subtle" style={{ margin: "6px 0 0", fontSize: 12 }}>{blocked}</p>
      )}
      {error && <p className="form-error" style={{ margin: "6px 0 0" }}>{error}</p>}
      {result && result.status !== "succeeded" && (
        <div className="form-error" style={{ margin: "6px 0 0" }}>
          <strong>{result.status}</strong>
          {result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
    </div>
  );
}
