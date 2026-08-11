import { apiGet } from "@/lib/api";
import type { Connector, ListResponse } from "@/lib/types";
import { ConnectorCard } from "@/components/connectors/ConnectorCard";

export default async function ConnectorsPage() {
  let connectors: Connector[] = [];
  let error: string | null = null;
  try {
    connectors = (await apiGet<ListResponse<Connector>>("/connectors")).items;
  } catch (e) {
    error = (e as Error).message;
  }

  // Group by server so each server's service managers sit together.
  const groups = new Map<string, { name: string; items: Connector[] }>();
  for (const c of connectors) {
    const g = groups.get(c.server_id) ?? { name: c.server_name || "Unknown server", items: [] };
    g.items.push(c);
    groups.set(c.server_id, g);
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Connectors</h1>
          <p className="subtle">
            Service managers discovered on your servers: nginx, systemd, Docker, pm2, and more.
          </p>
        </div>
      </div>

      {error && (
        <div className="form-error">Could not reach the control plane: <code>{error}</code></div>
      )}

      {!error && connectors.length === 0 && (
        <div className="panel">
          <div className="empty">
            No connectors discovered yet. They appear automatically once an agent reports what is
            installed on its server.
          </div>
        </div>
      )}

      {[...groups.entries()].map(([serverID, g]) => (
        <section key={serverID} style={{ marginBottom: 24 }}>
          <h2>{g.name}</h2>
          <div className="cards">
            {g.items.map((c) => <ConnectorCard key={c.id} connector={c} />)}
          </div>
        </section>
      ))}
    </>
  );
}
