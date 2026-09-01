import type { ConnectorOperation } from "@/lib/types";
import { StepList } from "./StepList";

const tone: Record<string, string> = {
  succeeded: "ok",
  pending: "info",
  failed: "danger",
  invalid: "warn",
  unauthorized: "warn",
  unsupported: "neutral",
  timeout: "warn",
  offline: "neutral",
};

/**
 * What has been done to this connector, newest first. Unlike the discovery cache
 * above it, these rows are append-only, so they still describe an apply that has since
 * been rolled back.
 */
export function OperationHistory({ items }: { items: ConnectorOperation[] }) {
  if (items.length === 0) {
    return (
      <div className="panel"><div className="empty">Nothing has been changed on this connector yet.</div></div>
    );
  }
  return (
    <div className="stack" style={{ gap: 10 }}>
      {items.map((op) => (
        <div key={op.id} className="panel" style={{ padding: 14 }}>
          <div className="row">
            <div className="cluster" style={{ gap: 8 }}>
              <span className={`status ${tone[op.status] ?? "neutral"}`}>{op.status}</span>
              <strong>{op.action ? `${op.op}: ${op.action}` : op.op}</strong>
              {op.dry_run && <span className="pill">dry run</span>}
              {op.ref && <span className="mono subtle" style={{ fontSize: 12 }}>{op.ref}</span>}
            </div>
            <span className="faint" style={{ fontSize: 12 }}>
              {new Date(op.created_at).toLocaleString()}
            </span>
          </div>
          {op.message && <p className="subtle" style={{ fontSize: 13, marginTop: 8 }}>{op.message}</p>}
          <StepList steps={op.steps ?? []} />
        </div>
      ))}
    </div>
  );
}
