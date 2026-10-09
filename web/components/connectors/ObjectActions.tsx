"use client";

import { useState } from "react";
import type { ConnectorResource } from "@/lib/types";
import { StepList } from "./StepList";
import { useConnectorCommand } from "./useConnectorCommand";
import { ProcessLogsPanel } from "./ProcessLogsPanel";

// Per-process verbs. Daemon-wide pm2 save/startup live on Pm2DaemonActions.
const ACTIONS: Record<string, string[]> = {
  systemd: ["start", "stop", "restart", "reload", "enable", "disable"],
  docker: ["start", "stop", "restart"],
  pm2: ["start", "stop", "restart", "reload", "delete", "flush"],
  nginx: ["start", "stop", "restart", "reload"],
};

const LOGS_KINDS = new Set(["pm2", "systemd", "docker"]);

const DESTRUCTIVE = new Set(["stop", "disable", "delete", "flush"]);

const LABELS: Record<string, string> = {
  start: "start",
  stop: "stop",
  restart: "restart",
  reload: "reload",
  enable: "enable",
  disable: "disable",
  delete: "delete",
  flush: "flush logs",
};

export function ObjectActions({
  connectorId,
  kind,
  resource,
  enabled,
}: {
  connectorId: string;
  kind: string;
  resource: ConnectorResource;
  enabled: boolean;
}) {
  const { busy, error, result, send } = useConnectorCommand();
  const [logs, setLogs] = useState<string | null>(null);
  const [logsBusy, setLogsBusy] = useState(false);
  const actions = ACTIONS[kind] ?? ["start", "stop", "restart"];

  async function run(action: string) {
    if (DESTRUCTIVE.has(action)) {
      const okToGo = window.confirm(`${LABELS[action] || action} ${resource.name}?`);
      if (!okToGo) return;
    }
    await send(`/api/connectors/${connectorId}/actions`, {
      method: "POST",
      body: JSON.stringify({ action, ref: resource.ref }),
    });
  }

  async function viewLogs() {
    setLogsBusy(true);
    try {
      const res = await fetch(
        `/api/connectors/${encodeURIComponent(connectorId)}/objects/${encodeURIComponent(resource.ref)}/logs`,
      );
      const body = (await res.json().catch(() => null)) as
        | { logs?: string; error?: { message?: string } }
        | null;
      if (!res.ok) throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      setLogs(body?.logs || "(no output)");
    } catch (e) {
      setLogs((e as Error).message);
    } finally {
      setLogsBusy(false);
    }
  }

  return (
    <div className="stack" style={{ gap: 6 }}>
      <div className="cluster" style={{ gap: 6, flexWrap: "wrap" }}>
        {actions.map((a) => (
          <button
            key={a}
            type="button"
            className={`button sm ${DESTRUCTIVE.has(a) ? "danger" : "secondary"}`}
            disabled={busy || !enabled}
            onClick={() => void run(a)}
            title={enabled ? `${LABELS[a] || a} ${resource.name}` : "The agent cannot drive this connector"}
          >
            {LABELS[a] || a}
          </button>
        ))}
        {LOGS_KINDS.has(kind) && (
          <button
            type="button"
            className="button secondary sm"
            disabled={logsBusy || !enabled}
            onClick={() => void viewLogs()}
            title={enabled ? `logs ${resource.name}` : "The agent cannot drive this connector"}
          >
            {logsBusy ? "…" : "logs"}
          </button>
        )}
      </div>
      {error && <p className="form-error" style={{ margin: 0 }}>{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"} style={{ margin: 0 }}>
          <strong>{result.status}</strong>
          {result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
      {logs !== null && (
        <ProcessLogsPanel
          title={resource.name}
          logs={logs}
          busy={logsBusy}
          onClose={() => setLogs(null)}
          onRefresh={() => void viewLogs()}
        />
      )}
    </div>
  );
}
