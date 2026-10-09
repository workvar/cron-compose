"use client";

import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import type { DeployInventory, DeployInventoryKind, ConnectorResource } from "@/lib/types";
import { ConnectorTabs } from "@/components/connectors/ConnectorTabs";
import { ObjectActions } from "@/components/connectors/ObjectActions";
import { Pm2DaemonActions } from "@/components/connectors/Pm2DaemonActions";
import { ProcessStateBadge } from "@/components/connectors/ProcessStateBadge";
import { ProcessImportPanel } from "./ProcessImportPanel";

const LABELS: Record<string, string> = {
  pm2: "PM2",
  systemd: "systemd",
  docker: "Docker",
};

type Props = {
  serverId: string;
  inventory: DeployInventory;
};

function KindTable({
  serverId,
  bucket,
  onImport,
}: {
  serverId: string;
  bucket: DeployInventoryKind;
  onImport: (r: ConnectorResource) => void;
}) {
  const router = useRouter();
  const primaryConn = bucket.connectors[0];
  const canAct = Boolean(primaryConn?.capabilities?.can_lifecycle !== false && primaryConn?.id);

  if (bucket.connectors.length === 0) {
    return (
      <div className="panel" style={{ marginTop: 12 }}>
        <div className="empty">
          No {LABELS[bucket.kind] || bucket.kind} connector discovered on this server yet.
          {serverId ? " Wait for the agent to report, or open Connectors." : null}
        </div>
      </div>
    );
  }

  return (
    <div className="stack" style={{ gap: 12, marginTop: 12 }}>
      {bucket.kind === "pm2" && primaryConn && (
        <div className="panel" style={{ padding: 14 }}>
          <div className="card-title" style={{ marginBottom: 8 }}>
            PM2 daemon
          </div>
          <p className="subtle" style={{ margin: "0 0 10px", fontSize: 13 }}>
            Persist the process list and install the boot hook so apps return after reboot.
          </p>
          <Pm2DaemonActions connectorId={primaryConn.id} enabled={canAct} />
        </div>
      )}

      {bucket.objects.length === 0 ? (
        <div className="panel">
          <div className="empty">No processes reported for {LABELS[bucket.kind] || bucket.kind}.</div>
        </div>
      ) : (
        <div className="panel" style={{ padding: 0, overflow: "auto" }}>
          <div className="cluster" style={{ justifyContent: "space-between", padding: "12px 14px" }}>
            <div className="subtle" style={{ fontSize: 13 }}>
              {bucket.objects.length} process{bucket.objects.length === 1 ? "" : "es"} · status from the agent
            </div>
            <button type="button" className="button secondary sm" onClick={() => router.refresh()}>
              Refresh
            </button>
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Restarts</th>
                <th>Command</th>
                <th>Cwd</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {bucket.objects.map((o) => {
                const a = o.attributes || {};
                const connectorId = o.connector_id || a.connector_id || primaryConn?.id || "";
                const rowCanAct = Boolean(
                  connectorId &&
                    (bucket.connectors.find((c) => c.id === connectorId) ?? primaryConn)
                      ?.capabilities?.can_lifecycle !== false,
                );
                return (
                  <tr key={`${o.connector_id}-${o.ref}`}>
                    <td>
                      <strong>{o.name}</strong>
                      <div className="subtle" style={{ fontSize: 11 }}>
                        id {o.ref}
                      </div>
                    </td>
                    <td>
                      <ProcessStateBadge state={o.state} />
                    </td>
                    <td className="subtle" style={{ fontSize: 12 }}>
                      {a.restarts ?? "—"}
                    </td>
                    <td className="subtle" style={{ fontSize: 12, maxWidth: 280 }}>
                      <code
                        style={{
                          whiteSpace: "nowrap",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          display: "block",
                        }}
                      >
                        {a.command || a.exec || "—"}
                      </code>
                    </td>
                    <td className="subtle" style={{ fontSize: 12 }}>
                      {a.cwd || "—"}
                    </td>
                    <td>
                      <div className="stack" style={{ gap: 8, alignItems: "flex-start" }}>
                        {connectorId ? (
                          <ObjectActions
                            connectorId={connectorId}
                            kind={bucket.kind}
                            resource={o}
                            enabled={rowCanAct}
                          />
                        ) : null}
                        <button type="button" className="button secondary sm" onClick={() => onImport(o)}>
                          Import as project
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export function ProcessManagerTabs({ serverId, inventory }: Props) {
  const [importing, setImporting] = useState<{ kind: string; resource: ConnectorResource } | null>(null);

  const tabs: { id: string; label: string; content: ReactNode }[] = inventory.items.map((bucket) => ({
    id: bucket.kind,
    label: `${LABELS[bucket.kind] || bucket.kind} (${bucket.count})`,
    content: (
      <KindTable
        serverId={serverId}
        bucket={bucket}
        onImport={(resource: ConnectorResource) => setImporting({ kind: bucket.kind, resource })}
      />
    ),
  }));

  return (
    <div>
      {importing && (
        <div style={{ marginBottom: 18 }}>
          <ProcessImportPanel
            serverId={serverId}
            kind={importing.kind}
            resource={importing.resource}
            onClose={() => setImporting(null)}
          />
        </div>
      )}
      <ConnectorTabs tabs={tabs} />
    </div>
  );
}
