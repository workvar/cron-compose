"use client";

import type { ConnectorSnapshot } from "@/lib/types";
import { StepList } from "./StepList";
import { useConnectorCommand } from "./useConnectorCommand";

/**
 * Config backups, newest first, each restorable in one click.
 *
 * Restoring is itself snapshotted server-side before it runs, so an accidental
 * restore is undoable by restoring the snapshot the restore created.
 */
export function SnapshotList({
  connectorId,
  items,
}: {
  connectorId: string;
  items: ConnectorSnapshot[];
}) {
  const { busy, error, result, send } = useConnectorCommand();

  async function restore(snap: ConnectorSnapshot) {
    const okToGo = window.confirm(
      `Restore ${snap.ref} to the version saved ${new Date(snap.created_at).toLocaleString()}?`,
    );
    if (!okToGo) return;
    await send(`/api/connectors/${connectorId}/snapshots/${snap.id}/restore`, { method: "POST" });
  }

  if (items.length === 0) {
    return <div className="panel"><div className="empty">No config backups yet. One is taken automatically before every apply.</div></div>;
  }

  return (
    <div className="stack" style={{ gap: 10 }}>
      {items.map((s) => (
        <div key={s.id} className="panel row" style={{ padding: 14 }}>
          <div>
            <div className="mono" style={{ fontSize: 13 }}>{s.ref}</div>
            <div className="faint" style={{ fontSize: 12, marginTop: 4 }}>
              {new Date(s.created_at).toLocaleString()} · {s.size_bytes} bytes
              {s.checksum ? ` · ${s.checksum.slice(0, 12)}` : ""}
            </div>
          </div>
          <button type="button" className="button secondary sm" disabled={busy} onClick={() => void restore(s)}>
            Restore
          </button>
        </div>
      ))}
      {error && <p className="form-error">{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"}>
          <strong>{result.status}</strong>{result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
    </div>
  );
}
