import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Connector, ConnectorResource, ListResponse } from "@/lib/types";
import { IconChevronLeft } from "@/components/icons";
import { ResourceTable } from "@/components/connectors/ResourceTable";

const tone: Record<Connector["status"], string> = {
  running: "ok",
  stopped: "danger",
  degraded: "warn",
  unknown: "neutral",
};

type Props = { params: Promise<{ id: string }> };

export default async function ConnectorDetailPage({ params }: Props) {
  const { id } = await params;
  let connector: Connector | null = null;
  let resources: ConnectorResource[] = [];
  let error: string | null = null;
  try {
    connector = await apiGet<Connector>(`/connectors/${id}`);
    resources = (await apiGet<ListResponse<ConnectorResource>>(`/connectors/${id}/resources`)).items;
  } catch (e) {
    error = (e as Error).message;
  }

  if (error || !connector) {
    return (
      <>
        <Link href="/connectors" className="back-link"><IconChevronLeft /> Connectors</Link>
        <div className="form-error">Could not load connector: <code>{error ?? "not found"}</code></div>
      </>
    );
  }

  const c = connector;
  const caps = Object.entries(c.capabilities || {})
    .filter(([, v]) => v)
    .map(([k]) => k.replace(/_/g, " "));
  const objects = resources.filter((r) => r.type === "object");
  const files = resources.filter((r) => r.type === "config_file");

  return (
    <>
      <Link href="/connectors" className="back-link"><IconChevronLeft /> Connectors</Link>
      <div className="page-head">
        <div>
          <h1>{c.kind}{c.instance ? ` (${c.instance})` : ""}</h1>
          <div className="cluster" style={{ marginTop: 6 }}>
            <span className={`status ${tone[c.status]}`}>{c.status}</span>
            {c.version && <span className="pill">v{c.version}</span>}
            <span className="pill">{c.manageable ? "manageable" : "read-only"}</span>
            {c.object_count > 0 && <span className="pill">{c.object_count} objects</span>}
            {c.last_seen_at && <span className="pill">seen {new Date(c.last_seen_at).toLocaleString()}</span>}
          </div>
        </div>
      </div>

      {!c.manageable && (
        <div className="panel" style={{ marginBottom: 16 }}>
          <div className="subtle" style={{ fontSize: 13 }}>
            Detected but not manageable: the agent lacks the privilege to change this connector.
            Editing and lifecycle actions arrive in a later phase and require granting the agent
            access. See docs/connectors.md.
          </div>
        </div>
      )}

      {caps.length > 0 && (
        <div className="cluster" style={{ marginBottom: 18 }}>
          {caps.map((cap) => <span key={cap} className="pill">{cap}</span>)}
        </div>
      )}

      {c.config_paths?.length > 0 && (
        <p className="faint" style={{ fontSize: 12, marginBottom: 16 }}>
          Config: {c.config_paths.join(" · ")}
        </p>
      )}

      {objects.length > 0 && (
        <>
          <h2>Objects</h2>
          <ResourceTable rows={objects} kind="object" />
        </>
      )}

      {files.length > 0 && (
        <>
          <h2>Config files</h2>
          <ResourceTable rows={files} kind="config_file" />
        </>
      )}

      {resources.length === 0 && (
        <div className="panel"><div className="empty">No resources reported for this connector.</div></div>
      )}
    </>
  );
}
