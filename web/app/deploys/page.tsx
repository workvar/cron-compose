import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { DeployProject, ListResponse, Server } from "@/lib/types";
import { IconGit, IconPlus } from "@/components/icons";
import { HealthBadge } from "@/components/deploys/HealthBadge";

export default async function DeploysPage() {
  let items: DeployProject[] = [];
  let servers: Server[] = [];
  let error: string | null = null;
  try {
    const [deploys, serversData] = await Promise.all([
      apiGet<ListResponse<DeployProject>>("/deploys"),
      apiGet<ListResponse<Server>>("/servers"),
    ]);
    items = deploys.items;
    servers = serversData.items;
  } catch (e) {
    error = (e as Error).message;
  }

  const serverName = (id: string) => servers.find((s) => s.id === id)?.name || id.slice(0, 8);

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Deploy</h1>
          <p className="subtle">Import a GitHub or GitLab repo onto an agent and run its installer.</p>
        </div>
        <div className="page-head-actions">
          <Link href="/deploys/new" className="button"><IconPlus /> Import git</Link>
        </div>
      </div>

      {error && (
        <div className="form-error">Could not load deploys: <code>{error}</code></div>
      )}

      {!error && items.length === 0 && (
        <div className="panel">
          <div className="empty">
            No deploys yet. <Link href="/deploys/new">Import a repository</Link> to clone it onto a server.
          </div>
        </div>
      )}

      <div className="stack">
        {items.map((p) => (
          <Link href={`/deploys/${p.id}`} className="panel" key={p.id}>
            <div className="row">
              <div className="cluster" style={{ flexWrap: "nowrap" }}>
                <span className="mini-icon"><IconGit /></span>
                <div>
                  <div style={{ fontWeight: 700, color: "var(--text)" }}>{p.name}</div>
                  <div className="subtle" style={{ fontSize: 13 }}>
                    {p.provider}/{p.repo_full_name} · {serverName(p.server_id)}
                  </div>
                </div>
              </div>
              <div className="cluster">
                <HealthBadge state={p.health_state} />
                <span className="pill">{p.language}</span>
                <span className="pill">{p.process_manager}</span>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </>
  );
}
