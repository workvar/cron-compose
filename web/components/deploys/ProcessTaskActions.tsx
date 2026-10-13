"use client";

import { useState } from "react";
import { IconPause, IconPlay, IconRestart, IconStop } from "@/components/icons";
import { useConnectorCommand } from "@/components/connectors/useConnectorCommand";
import { StepList } from "@/components/connectors/StepList";
import { isProcessPaused, type ProcessRow } from "@/lib/project-processes";

type Props = {
  row: ProcessRow | null;
  /** Soft-disable while deploy is still writing the process list. */
  enabled?: boolean;
  onChanged?: () => void;
};

/**
 * Compact Pause / Stop / Restart controls for one process task card.
 * Pause toggles to Resume when the process is already frozen.
 */
export function ProcessTaskActions({ row, enabled = true, onChanged }: Props) {
  const { busy, error, result, send } = useConnectorCommand();
  const [localPaused, setLocalPaused] = useState(false);

  if (!row || row.pm === "none") return null;

  const resource = row.resource;
  const paused = localPaused || isProcessPaused(resource?.state);
  const ready = Boolean(enabled && row.connectorId && resource);
  const label = row.label || row.processName;

  async function run(action: string) {
    if (!row?.connectorId || !row.resource) return;
    if (action === "stop") {
      const ok = window.confirm(`Stop ${label}?`);
      if (!ok) return;
    }
    const body = await send(`/api/connectors/${encodeURIComponent(row.connectorId)}/actions`, {
      method: "POST",
      body: JSON.stringify({ action, ref: row.resource.ref }),
    });
    if (body?.status === "succeeded") {
      if (action === "pause") setLocalPaused(true);
      else if (action === "unpause" || action === "restart" || action === "stop" || action === "start") {
        setLocalPaused(false);
      }
      onChanged?.();
    }
  }

  return (
    <div className="deploy-proc-actions">
      <div className="deploy-proc-actions-btns" role="group" aria-label={`${label} process controls`}>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={busy || !ready}
          title={
            !ready
              ? "Process not found on the server yet"
              : paused
                ? `Resume ${label}`
                : `Pause ${label}`
          }
          onClick={() => void run(paused ? "unpause" : "pause")}
        >
          {paused ? <IconPlay /> : <IconPause />}
          <span>{paused ? "Resume" : "Pause"}</span>
        </button>
        <button
          type="button"
          className="button danger sm deploy-proc-action"
          disabled={busy || !ready}
          title={!ready ? "Process not found on the server yet" : `Stop ${label}`}
          onClick={() => void run("stop")}
        >
          <IconStop />
          <span>Stop</span>
        </button>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={busy || !ready}
          title={!ready ? "Process not found on the server yet" : `Restart ${label}`}
          onClick={() => void run("restart")}
        >
          <IconRestart />
          <span>Restart</span>
        </button>
      </div>
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
