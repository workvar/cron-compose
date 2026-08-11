import Link from "next/link";
import type { Connector } from "@/lib/types";
import { IconPlug } from "@/components/icons";

const tone: Record<Connector["status"], string> = {
  running: "ok",
  stopped: "danger",
  degraded: "warn",
  unknown: "neutral",
};

export function ConnectorCard({ connector }: { connector: Connector }) {
  const c = connector;
  return (
    <Link href={`/connectors/${c.id}`} className="panel">
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div className="cluster" style={{ flexWrap: "nowrap" }}>
          <span className="mini-icon"><IconPlug /></span>
          <div>
            <div style={{ fontWeight: 700, fontSize: 15, color: "var(--text)" }}>
              {c.kind}{c.instance ? ` (${c.instance})` : ""}
            </div>
            <div className="subtle" style={{ fontSize: 12 }}>
              {c.version ? `v${c.version}` : "version unknown"}
              {c.object_count > 0 ? ` · ${c.object_count} objects` : ""}
            </div>
          </div>
        </div>
        <span className={`status ${tone[c.status]}`}>{c.status}</span>
      </div>
      <div className="faint" style={{ fontSize: 12, marginTop: 12 }}>
        {c.manageable ? "manageable" : "read-only"}
        {c.config_paths?.length ? ` · ${c.config_paths.length} config paths` : ""}
      </div>
    </Link>
  );
}
