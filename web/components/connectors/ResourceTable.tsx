import type { ConnectorResource } from "@/lib/types";

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

function formatSize(n?: number): string {
  if (!n || n <= 0) return "n/a";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function ResourceTable({
  rows,
  kind,
}: {
  rows: ConnectorResource[];
  kind: "object" | "config_file";
}) {
  return (
    <div className="panel" style={{ marginBottom: 18, padding: 0, overflow: "hidden" }}>
      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>{kind === "object" ? "State" : "Size"}</th>
            <th>{kind === "object" ? "Reference" : "Path"}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td style={{ fontWeight: 600 }}>{r.name}</td>
              {kind === "object" ? (
                <td>
                  <span className={`status ${stateTone[r.state ?? ""] ?? "neutral"}`}>
                    {r.state || "unknown"}
                  </span>
                </td>
              ) : (
                <td className="subtle">{formatSize(r.size_bytes)}</td>
              )}
              <td className="mono subtle" style={{ fontSize: 12 }}>{r.ref}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
