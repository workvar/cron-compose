"use client";

import { useState, type ReactNode } from "react";
import type { DeployInventory, DeployInventoryKind, ConnectorResource } from "@/lib/types";
import { ConnectorTabs } from "@/components/connectors/ConnectorTabs";
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
  if (bucket.objects.length === 0) {
    return (
      <div className="panel" style={{ marginTop: 12 }}>
        <div className="empty">No running processes reported for {LABELS[bucket.kind] || bucket.kind}.</div>
      </div>
    );
  }

  return (
    <div className="panel" style={{ marginTop: 12, padding: 0, overflow: "auto" }}>
      <table className="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>State</th>
            <th>Command</th>
            <th>Cwd</th>
            <th>Env</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {bucket.objects.map((o) => {
            const a = o.attributes || {};
            const envCount = a.env_count || (a.env_keys ? String(a.env_keys.split(",").filter(Boolean).length) : "0");
            return (
              <tr key={`${o.connector_id}-${o.ref}`}>
                <td><strong>{o.name}</strong></td>
                <td>
                  <span className={`status ${o.state === "online" || o.state === "running" ? "ok" : "neutral"}`}>
                    {o.state || "—"}
                  </span>
                </td>
                <td className="subtle" style={{ fontSize: 12, maxWidth: 280 }}>
                  <code style={{ whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", display: "block" }}>
                    {a.command || a.exec || "—"}
                  </code>
                </td>
                <td className="subtle" style={{ fontSize: 12 }}>{a.cwd || "—"}</td>
                <td className="subtle" style={{ fontSize: 12 }}>{envCount} vars</td>
                <td style={{ textAlign: "right" }}>
                  <button type="button" className="button secondary sm" onClick={() => onImport(o)}>
                    Import
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
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
