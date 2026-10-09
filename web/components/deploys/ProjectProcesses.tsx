"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ConnectorResource, DeployApp, DeployInventory, DeployProject } from "@/lib/types";
import { qualifyProcessName } from "@/lib/process-name";
import { useConnectorCommand } from "@/components/connectors/useConnectorCommand";
import { StepList } from "@/components/connectors/StepList";
import { ProcessStateBadge } from "@/components/connectors/ProcessStateBadge";
import { ProcessLogsPanel } from "@/components/connectors/ProcessLogsPanel";
import { Pm2DaemonActions } from "@/components/connectors/Pm2DaemonActions";

type Row = {
  label: string;
  processName: string;
  pm: string;
  resource: ConnectorResource | null;
  connectorId: string | null;
};

const ROW_ACTIONS = ["start", "stop", "restart", "reload", "delete"] as const;
const DESTRUCTIVE = new Set(["stop", "delete", "flush"]);

function buildRows(project: DeployProject, inventory: DeployInventory | null): Row[] {
  const apps: DeployApp[] =
    project.apps?.length > 0
      ? project.apps
      : [
          {
            name: project.name,
            root: project.root_directory || ".",
            process_manager: project.process_manager,
            process_name: qualifyProcessName(project.name, project.name),
          },
        ];

  const objects: { kind: string; connectorId: string; resource: ConnectorResource }[] = [];
  for (const bucket of inventory?.items ?? []) {
    const fallbackConn = bucket.connectors[0]?.id ?? "";
    for (const o of bucket.objects) {
      objects.push({
        kind: bucket.kind,
        connectorId: o.connector_id || o.attributes?.connector_id || fallbackConn,
        resource: o,
      });
    }
  }

  return apps.map((app) => {
    const pm = (app.process_manager || project.process_manager || "none").trim() || "none";
    const processName =
      app.process_name || qualifyProcessName(project.name, app.name || app.root || "app");
    const hit =
      objects.find(
        (o) =>
          o.kind === pm &&
          (o.resource.name === processName ||
            o.resource.ref === processName ||
            o.resource.name === app.name),
      ) ?? null;
    return {
      label: app.name || processName,
      processName,
      pm,
      resource: hit?.resource ?? null,
      connectorId: hit?.connectorId ?? null,
    };
  });
}

export function ProjectProcesses({ project }: { project: DeployProject }) {
  const [inventory, setInventory] = useState<DeployInventory | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [logsFor, setLogsFor] = useState<string | null>(null);
  const [logsText, setLogsText] = useState<string>("");
  const [logsBusy, setLogsBusy] = useState(false);
  const [logsRow, setLogsRow] = useState<Row | null>(null);
  const { busy, error, result, send } = useConnectorCommand();

  const refresh = useCallback(async () => {
    setLoadError(null);
    try {
      const res = await fetch(`/api/servers/${encodeURIComponent(project.server_id)}/deploy-inventory`);
      if (!res.ok) throw new Error(`Could not load process inventory (${res.status})`);
      setInventory((await res.json()) as DeployInventory);
    } catch (e) {
      setLoadError((e as Error).message);
    }
  }, [project.server_id]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const rows = buildRows(project, inventory);
  const manageable = rows.some((r) => r.pm !== "none");
  const pm2ConnectorId = useMemo(() => {
    const bucket = inventory?.items.find((b) => b.kind === "pm2");
    return bucket?.connectors[0]?.id || rows.find((r) => r.pm === "pm2")?.connectorId || null;
  }, [inventory, rows]);
  const usesPm2 = rows.some((r) => r.pm === "pm2") || Boolean(pm2ConnectorId);

  if (!manageable) return null;

  async function runAction(row: Row, action: string) {
    if (!row.connectorId || !row.resource) return;
    if (DESTRUCTIVE.has(action)) {
      const ok = window.confirm(`${action} ${row.processName}?`);
      if (!ok) return;
    }
    await send(`/api/connectors/${encodeURIComponent(row.connectorId)}/actions`, {
      method: "POST",
      body: JSON.stringify({ action, ref: row.resource.ref }),
    });
    await refresh();
  }

  async function viewLogs(row: Row) {
    if (!row.connectorId || !row.resource) return;
    setLogsFor(row.processName);
    setLogsRow(row);
    setLogsBusy(true);
    setLogsText("");
    try {
      const res = await fetch(
        `/api/connectors/${encodeURIComponent(row.connectorId)}/objects/${encodeURIComponent(row.resource.ref)}/logs`,
      );
      const body = (await res.json().catch(() => null)) as
        | { logs?: string; error?: { message?: string } }
        | null;
      if (!res.ok) throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      setLogsText(body?.logs || "(no output)");
    } catch (e) {
      setLogsText((e as Error).message);
    } finally {
      setLogsBusy(false);
    }
  }

  return (
    <div className="panel" style={{ marginTop: 18 }}>
      <div className="cluster" style={{ justifyContent: "space-between", alignItems: "baseline" }}>
        <div>
          <div className="card-title">Processes</div>
          <p className="subtle" style={{ margin: "6px 0 0" }}>
            Live status from the agent. Start, stop, restart, view or download logs without redeploying.
          </p>
        </div>
        <button type="button" className="button secondary sm" onClick={() => void refresh()} disabled={busy}>
          Refresh status
        </button>
      </div>

      {usesPm2 && pm2ConnectorId && (
        <div style={{ marginTop: 14, paddingTop: 12, borderTop: "1px solid var(--border)" }}>
          <div className="card-title" style={{ marginBottom: 8, fontSize: 13 }}>
            PM2 daemon
          </div>
          <Pm2DaemonActions connectorId={pm2ConnectorId} enabled />
        </div>
      )}

      {loadError && <p className="form-error">{loadError}</p>}

      <div style={{ marginTop: 14, overflow: "auto" }}>
        <table className="data-table">
          <thead>
            <tr>
              <th>App</th>
              <th>Process</th>
              <th>Manager</th>
              <th>Status</th>
              <th>Restarts</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const attrs = row.resource?.attributes || {};
              const actions =
                row.pm === "pm2" ? ROW_ACTIONS : (["start", "stop", "restart"] as const);
              return (
                <tr key={row.processName}>
                  <td>
                    <strong>{row.label}</strong>
                  </td>
                  <td>
                    <code style={{ fontSize: 12 }}>{row.processName}</code>
                  </td>
                  <td className="subtle">{row.pm}</td>
                  <td>
                    {row.resource ? (
                      <ProcessStateBadge state={row.resource.state} />
                    ) : (
                      <span className="subtle">not found</span>
                    )}
                  </td>
                  <td className="subtle" style={{ fontSize: 12 }}>
                    {attrs.restarts ?? "—"}
                  </td>
                  <td style={{ textAlign: "right" }}>
                    <div className="cluster" style={{ gap: 6, justifyContent: "flex-end", flexWrap: "wrap" }}>
                      {actions.map((a) => (
                        <button
                          key={a}
                          type="button"
                          className={`button sm ${DESTRUCTIVE.has(a) ? "danger" : "secondary"}`}
                          disabled={busy || !row.connectorId || !row.resource || row.pm === "none"}
                          onClick={() => void runAction(row, a)}
                        >
                          {a}
                        </button>
                      ))}
                      <button
                        type="button"
                        className="button secondary sm"
                        disabled={logsBusy || !row.connectorId || !row.resource}
                        onClick={() => void viewLogs(row)}
                      >
                        logs
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {error && <p className="form-error">{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"} style={{ marginTop: 10 }}>
          <strong>{result.status}</strong>
          {result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}

      {logsFor && (
        <div style={{ marginTop: 14 }}>
          <ProcessLogsPanel
            title={logsFor}
            logs={logsText}
            busy={logsBusy}
            onClose={() => {
              setLogsFor(null);
              setLogsRow(null);
            }}
            onRefresh={logsRow ? () => void viewLogs(logsRow) : undefined}
          />
        </div>
      )}
    </div>
  );
}
