import type { ConnectorResource } from "@/lib/types";
import { ObjectActions } from "./ObjectActions";

const stateTone: Record<string, string> = {
  running: "ok",
  active: "ok",
  online: "ok",
  enabled: "info",
  stopped: "danger",
  failed: "danger",
  errored: "danger",
  inactive: "neutral",
};

/**
 * The objects table with a lifecycle control per row. Rendered instead of the plain
 * ResourceTable when the viewer is at least an operator and the connector reported
 * that the agent can actually drive it.
 */
export function ObjectTable({
  connectorId,
  kind,
  rows,
  canAct,
}: {
  connectorId: string;
  kind: string;
  rows: ConnectorResource[];
  canAct: boolean;
}) {
  return (
    <div className="panel" style={{ marginBottom: 18, padding: 0, overflow: "hidden" }}>
      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>State</th>
            <th>Reference</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td style={{ fontWeight: 600 }}>{r.name}</td>
              <td>
                <span className={`status ${stateTone[r.state ?? ""] ?? "neutral"}`}>
                  {r.state || "unknown"}
                </span>
              </td>
              <td className="mono subtle" style={{ fontSize: 12 }}>{r.ref}</td>
              <td>
                <ObjectActions
                  connectorId={connectorId}
                  kind={kind}
                  resource={r}
                  enabled={canAct}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
