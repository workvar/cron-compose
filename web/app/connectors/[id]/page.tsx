import Link from "next/link";
import { apiGet } from "@/lib/api";
import type {
  Connector,
  ConnectorOperation,
  ConnectorResource,
  ConnectorSnapshot,
  ListResponse,
  Me,
} from "@/lib/types";
import { IconChevronLeft } from "@/components/icons";
import { ResourceTable } from "@/components/connectors/ResourceTable";
import { ObjectTable } from "@/components/connectors/ObjectTable";
import { ConfigEditor } from "@/components/connectors/ConfigEditor";
import { OperationHistory } from "@/components/connectors/OperationHistory";
import { SnapshotList } from "@/components/connectors/SnapshotList";
import { ConnectorTabs } from "@/components/connectors/ConnectorTabs";
import { PortsPanel } from "@/components/connectors/PortsPanel";

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
  let operations: ConnectorOperation[] = [];
  let me: Me | null = null;
  let error: string | null = null;

  try {
    connector = await apiGet<Connector>(`/connectors/${id}`);
    [resources, operations, me] = await Promise.all([
      apiGet<ListResponse<ConnectorResource>>(`/connectors/${id}/resources`).then((r) => r.items),
      apiGet<ListResponse<ConnectorOperation>>(`/connectors/${id}/operations?limit=20`).then((r) => r.items),
      apiGet<Me>("/me"),
    ]);
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
  const isAdmin = me?.role === "admin" || me?.role === "owner";
  const isOperator = isAdmin || me?.role === "operator";
  const caps = c.capabilities ?? {};
  const objects = resources.filter((r) => r.type === "object");
  const files = resources.filter((r) => r.type === "config_file");

  let snapshots: ConnectorSnapshot[] = [];
  if (isAdmin && files.length > 0) {
    snapshots = await apiGet<ListResponse<ConnectorSnapshot>>(`/connectors/${id}/snapshots?limit=20`)
      .then((r) => r.items)
      .catch(() => []);
  }

  const canAct = isOperator && Boolean(caps.can_lifecycle);
  const canEdit = isAdmin && Boolean(caps.can_edit);
  const capLabels = Object.entries(caps).filter(([, v]) => v).map(([k]) => k.replace(/_/g, " "));
  const showPorts = c.kind === "systemd" || c.kind === "pm2";

  const tabs = [
    {
      id: "objects",
      label: objects.length > 0 ? `Objects (${objects.length})` : "Objects",
      content: objects.length > 0
        ? isOperator
          ? <ObjectTable connectorId={c.id} kind={c.kind} rows={objects} canAct={canAct} />
          : <ResourceTable rows={objects} kind="object" />
        : <div className="panel"><div className="empty">No objects reported for this connector.</div></div>,
    },
    ...(showPorts ? [{
      id: "ports",
      label: "Ports",
      content: <PortsPanel connectorId={c.id} canAct={canAct} />,
    }] : []),
    ...(files.length > 0 ? [{
      id: "config",
      label: "Config",
      content: (
        <>
          {isAdmin
            ? <ConfigEditor connectorId={c.id} files={files} canEdit={canEdit} />
            : <ResourceTable rows={files} kind="config_file" />}
          {isAdmin && snapshots.length > 0 && (
            <div style={{ marginTop: 18 }}>
              <h2>Backups</h2>
              <SnapshotList connectorId={c.id} items={snapshots} />
            </div>
          )}
        </>
      ),
    }] : []),
    {
      id: "history",
      label: "History",
      content: <OperationHistory items={operations} />,
    },
  ];

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
            Run the agent as root, add it to the relevant group, or grant a passwordless sudo
            entry for the tool. See docs/connectors.md.
          </div>
        </div>
      )}

      {capLabels.length > 0 && (
        <div className="cluster" style={{ marginBottom: 18 }}>
          {capLabels.map((cap) => <span key={cap} className="pill">{cap}</span>)}
        </div>
      )}

      {c.config_paths?.length > 0 && (
        <p className="faint" style={{ fontSize: 12, marginBottom: 16 }}>
          Config: {c.config_paths.join(" · ")}
        </p>
      )}

      <ConnectorTabs tabs={tabs} />
    </>
  );
}
