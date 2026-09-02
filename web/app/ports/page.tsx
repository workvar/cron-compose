"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import type { Connector, ConnectorPort, ListResponse, Me } from "@/lib/types";
import { filterMappedPorts, type MappedPort } from "@/lib/ui-helpers";
import { IconSearch } from "@/components/icons";
import { PortLabelInput } from "@/components/connectors/PortLabelInput";
import { StepList } from "@/components/connectors/StepList";
import { useConnectorCommand } from "@/components/connectors/useConnectorCommand";

const PORT_KINDS = new Set(["systemd", "pm2"]);

export default function PortsPage() {
  const [rows, setRows] = useState<MappedPort[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [canAct, setCanAct] = useState(false);
  const { busy, error, result, send } = useConnectorCommand();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const [meRes, connRes] = await Promise.all([
        fetch("/api/me"),
        fetch("/api/connectors"),
      ]);
      const me = (await meRes.json().catch(() => null)) as Me | null;
      const role = me?.role ?? "";
      setCanAct(role === "admin" || role === "owner" || role === "operator");

      const body = (await connRes.json().catch(() => null)) as
        | (ListResponse<Connector> & { error?: { message?: string } })
        | null;
      if (!connRes.ok) throw new Error(body?.error?.message ?? `HTTP ${connRes.status}`);
      const connectors = (body?.items ?? []).filter((c) => PORT_KINDS.has(c.kind));

      const groups = await Promise.all(connectors.map(async (c) => {
        const res = await fetch(`/api/connectors/${c.id}/ports`);
        const payload = (await res.json().catch(() => null)) as
          | (ListResponse<ConnectorPort> & { error?: { message?: string } })
          | null;
        if (!res.ok) return [] as MappedPort[];
        return (payload?.items ?? []).map((p): MappedPort => ({
          ...p,
          connector_id: c.id,
          server_id: c.server_id,
          server_name: c.server_name || c.server_id,
          kind: c.kind,
        }));
      }));
      setRows(groups.flat());
    } catch (e) {
      setRows([]);
      setLoadError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const visible = useMemo(() => filterMappedPorts(rows, query), [rows, query]);

  async function closePort(row: MappedPort) {
    const okToGo = window.confirm(
      `Stop ${row.name} to free ${row.address}:${row.port}?`,
    );
    if (!okToGo) return;
    const body = await send(`/api/connectors/${row.connector_id}/actions`, {
      method: "POST",
      body: JSON.stringify({ action: "stop", ref: row.ref }),
    });
    if (body?.status === "succeeded") void load();
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Ports</h1>
          <p className="subtle">
            Listening sockets owned by systemd and pm2 on your servers. Label them so you
            remember what each bind is for.
          </p>
        </div>
        <div className="page-head-actions">
          <button type="button" className="button secondary sm" onClick={() => void load()} disabled={loading || busy}>
            {loading ? "Loading…" : "Refresh"}
          </button>
        </div>
      </div>

      <div className="search ports-search">
        <IconSearch />
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search by label, port, process, owner, or server…"
          aria-label="Search ports"
        />
      </div>

      {loadError && <p className="form-error">{loadError}</p>}

      {!loading && !loadError && rows.length === 0 && (
        <div className="panel">
          <div className="empty">
            No listening ports yet. They appear once a systemd or pm2 connector is
            discovered. <Link href="/connectors">Open connectors</Link>
          </div>
        </div>
      )}

      {!loading && rows.length > 0 && visible.length === 0 && (
        <div className="panel"><div className="empty">No ports match “{query}”.</div></div>
      )}

      {visible.length > 0 && (
        <div className="panel" style={{ marginBottom: 0, padding: 0, overflow: "hidden" }}>
          <table className="table">
            <thead>
              <tr>
                <th>Server</th>
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
              {visible.map((row) => (
                <tr key={`${row.connector_id}-${row.pid}-${row.address}-${row.port}`}>
                  <td>
                    <Link href={`/connectors/${row.connector_id}`}>{row.server_name}</Link>
                    <div className="mono faint" style={{ fontSize: 12 }}>{row.kind}</div>
                  </td>
                  <td style={{ fontWeight: 600 }} className="mono">{row.port}</td>
                  <td>
                    <PortLabelInput
                      serverId={row.server_id}
                      row={row}
                      disabled={!canAct}
                      onSaved={(label) => setRows((prev) => prev.map((p) =>
                        p.connector_id === row.connector_id && p.address === row.address && p.port === row.port
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

      {error && <p className="form-error">{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"} style={{ margin: 0 }}>
          <strong>{result.status}</strong>
          {result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
    </>
  );
}
