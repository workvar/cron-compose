"use client";

import { useCallback, useEffect, useState } from "react";
import type { ConnectorPort, ListResponse } from "@/lib/types";
import { PortLabelInput } from "./PortLabelInput";
import { StepList } from "./StepList";
import { useConnectorCommand } from "./useConnectorCommand";

export function PortsPanel({
  connectorId,
  serverId,
  canAct,
}: {
  connectorId: string;
  serverId: string;
  canAct: boolean;
}) {
  const [items, setItems] = useState<ConnectorPort[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const { busy, error, result, send } = useConnectorCommand();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await fetch(`/api/connectors/${connectorId}/ports`);
      const body = (await res.json().catch(() => null)) as
        | (ListResponse<ConnectorPort> & { error?: { message?: string } })
        | null;
      if (!res.ok) throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      setItems(body?.items ?? []);
    } catch (e) {
      setItems([]);
      setLoadError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [connectorId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function closePort(row: ConnectorPort) {
    const okToGo = window.confirm(
      `Stop ${row.name} to free ${row.address}:${row.port}?`,
    );
    if (!okToGo) return;
    const body = await send(`/api/connectors/${connectorId}/actions`, {
      method: "POST",
      body: JSON.stringify({ action: "stop", ref: row.ref }),
    });
    if (body?.status === "succeeded") void load();
  }

  return (
    <div className="stack" style={{ gap: 12 }}>
      <div className="row">
        <p className="faint" style={{ margin: 0, fontSize: 13 }}>
          Listening TCP ports owned by this connector. Close stops the unit or process,
          it does not kill a raw PID.
        </p>
        <button type="button" className="button secondary sm" onClick={() => void load()} disabled={loading || busy}>
          {loading ? "Loading…" : "Refresh"}
        </button>
      </div>

      {loadError && <p className="form-error" style={{ margin: 0 }}>{loadError}</p>}

      {!loading && !loadError && items.length === 0 && (
        <div className="panel"><div className="empty">No listening ports reported for this connector.</div></div>
      )}

      {items.length > 0 && (
        <div className="panel" style={{ marginBottom: 0, padding: 0, overflow: "hidden" }}>
          <table className="table">
            <thead>
              <tr>
                <th>Port</th>
                <th>Label</th>
                <th>Address</th>
                <th>Process</th>
                <th>Owner</th>
                <th>PID</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={`${row.pid}-${row.address}-${row.port}`}>
                  <td style={{ fontWeight: 600 }} className="mono">{row.port}</td>
                  <td>
                    <PortLabelInput
                      serverId={serverId}
                      row={row}
                      disabled={!canAct}
                      onSaved={(label) => setItems((prev) => prev.map((p) =>
                        p.pid === row.pid && p.address === row.address && p.port === row.port
                          ? { ...p, label }
                          : p
                      ))}
                    />
                  </td>
                  <td className="mono subtle" style={{ fontSize: 12 }}>{row.address}</td>
                  <td>{row.process}</td>
                  <td>
                    <span style={{ fontWeight: 600 }}>{row.name}</span>
                    <div className="mono faint" style={{ fontSize: 12 }}>{row.ref}</div>
                  </td>
                  <td className="mono subtle" style={{ fontSize: 12 }}>{row.pid}</td>
                  <td>
                    <button
                      type="button"
                      className="button sm danger"
                      disabled={busy || loading || !canAct || row.protected}
                      onClick={() => void closePort(row)}
                      title={
                        row.protected
                          ? "This process is protected (PID 1 or the CronCompose agent)"
                          : canAct
                            ? `Stop ${row.name}`
                            : "The agent cannot drive this connector"
                      }
                    >
                      Close
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {error && <p className="form-error" style={{ margin: 0 }}>{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"} style={{ margin: 0 }}>
          <strong>{result.status}</strong>
          {result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
    </div>
  );
}
