"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { IconPause, IconPlay, IconRestart, IconStop } from "@/components/icons";
import { useConnectorCommand } from "@/components/connectors/useConnectorCommand";
import { StepList } from "@/components/connectors/StepList";
import { isProcessPaused, isProcessRunning, processActionRef, type ProcessRow } from "@/lib/project-processes";

type Props = {
  row: ProcessRow | null;
  /** Project id — required for Restart to redeploy the full install→run chain. */
  projectId?: string;
  /** App name as stored in the project flow. */
  appName?: string;
  /** Server that owns the process. Used when no connector row exists yet. */
  serverId?: string;
  /** Deploy user, so pm2 runs against that account's daemon. */
  runAsUser?: string;
  /** Soft-disable while deploy is still writing the process list. */
  enabled?: boolean;
  onChanged?: () => void;
};

/**
 * Start / Pause / Stop / Restart for one process card.
 * Restart starts a new deploy for that app from install, not a pm2 restart alone.
 */
export function ProcessTaskActions({
  row,
  projectId,
  appName,
  serverId,
  runAsUser,
  enabled = true,
  onChanged,
}: Props) {
  const router = useRouter();
  const { busy, error, result, send } = useConnectorCommand();
  const [redeployBusy, setRedeployBusy] = useState(false);
  const [redeployError, setRedeployError] = useState<string | null>(null);
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
  const direct = Boolean(serverId && ref);
  const ready = Boolean(enabled && ref && (row.connectorId || direct));
  const label = row.label || row.processName;
  const canRedeploy = Boolean(projectId && (appName || label));
  const blocked = !enabled
    ? "Wait for this deploy step to finish"
    : !ref
      ? "Process name is missing"
      : !row.connectorId && !direct
        ? `No ${row.pm} connector on this server yet — open Connectors`
        : "";

  async function run(action: string) {
    if (!ref || !row) return;
    if (action === "stop") {
      const ok = window.confirm(`Stop ${label}?`);
      if (!ok) return;
    }
    const path = serverId
      ? `/api/servers/${encodeURIComponent(serverId)}/process-actions`
      : row.connectorId
        ? `/api/connectors/${encodeURIComponent(row.connectorId)}/actions`
        : "";
    if (!path) return;
    const body = await send(path, {
      method: "POST",
      body: JSON.stringify(
        serverId
          ? { pm: row.pm, action, ref, run_as_user: runAsUser || "" }
          : { action, ref },
      ),
    });
    if (body?.status === "succeeded") {
      if (action === "pause") setForced("paused");
      else if (action === "stop") setForced("stopped");
      else if (action === "unpause" || action === "start") setForced("running");
      onChanged?.();
    }
  }

  async function redeployChain() {
    if (!projectId) return;
    const name = appName || label;
    const ok = window.confirm(
      `Redeploy ${name} from the start?\n\nThis runs install → build → cleanup → pre-start → run → start for ${name} only.`,
    );
    if (!ok) return;
    setRedeployBusy(true);
    setRedeployError(null);
    try {
      const res = await fetch(`/api/deploys/${encodeURIComponent(projectId)}/runs`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ apps: [name] }),
      });
      const body = (await res.json().catch(() => null)) as
        | { id?: string; error?: { message?: string }; skipped?: string }
        | null;
      if (!res.ok) {
        setRedeployError(body?.error?.message || `Could not start redeploy (${res.status})`);
        return;
      }
      if (body?.id) {
        router.push(`/deploys/runs/${body.id}`);
        return;
      }
      setRedeployError(body?.skipped || "Redeploy was skipped");
    } catch (e) {
      setRedeployError((e as Error).message || "Could not start redeploy");
    } finally {
      setRedeployBusy(false);
    }
  }

  const waiting = busy || redeployBusy;
  const canStart = ready && (!running || paused);
  const canPause = ready && running && !paused;
  const canStop = ready;
  const canRestart = canRedeploy && enabled;

  return (
    <div className="deploy-proc-actions">
      <div className="deploy-proc-actions-btns" role="group" aria-label={`${label} process controls`}>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={waiting || !canStart}
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
          disabled={waiting || !canPause}
          title={blocked || (running ? `Pause ${label}` : `${label} is not running`)}
          onClick={() => void run("pause")}
        >
          <IconPause />
          <span>Pause</span>
        </button>
        <button
          type="button"
          className="button danger sm deploy-proc-action"
          disabled={waiting || !canStop}
          title={blocked || `Stop ${label}`}
          onClick={() => void run("stop")}
        >
          <IconStop />
          <span>Stop</span>
        </button>
        <button
          type="button"
          className="button secondary sm deploy-proc-action"
          disabled={waiting || !canRestart}
          title={
            canRedeploy
              ? `Redeploy ${label} from install through run`
              : "Project is missing — cannot redeploy"
          }
          onClick={() => void redeployChain()}
        >
          <IconRestart />
          <span>{redeployBusy ? "Redeploying…" : "Restart"}</span>
        </button>
      </div>
      {!ready && blocked && (
        <p className="subtle" style={{ margin: "6px 0 0", fontSize: 12 }}>{blocked}</p>
      )}
      {(error || redeployError) && (
        <p className="form-error" style={{ margin: "6px 0 0" }}>{redeployError || error}</p>
      )}
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
