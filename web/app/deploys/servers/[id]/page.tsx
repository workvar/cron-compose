import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { DeployInventory, DeployProject, ListResponse, Server } from "@/lib/types";
import { IconChevronLeft, IconPlus } from "@/components/icons";
import { ServerEmojiBadge } from "@/components/ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";
import { ProcessManagerTabs } from "@/components/deploys/ProcessManagerTabs";

const tone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

type Props = { params: Promise<{ id: string }> };

export default async function DeployServerPage({ params }: Props) {
  const { id } = await params;
  let server: Server | null = null;
  let inventory: DeployInventory | null = null;
  let projects: DeployProject[] = [];
  let error: string | null = null;

  try {
    server = await apiGet<Server>(`/servers/${id}`);
    inventory = await apiGet<DeployInventory>(`/servers/${id}/deploy-inventory`);
    projects = (await apiGet<ListResponse<DeployProject>>("/deploys").catch(() => ({ items: [] as DeployProject[] })))
      .items.filter((p) => p.server_id === id);
  } catch (e) {
    error = (e as Error).message;
  }

  if (error || !server || !inventory) {
    return (
      <>
        <Link href="/deploys" className="back-link"><IconChevronLeft /> Deploy</Link>
        <div className="form-error">Could not load server: <code>{error ?? "not found"}</code></div>
      </>
    );
  }

  return (
    <>
      <Link href="/deploys" className="back-link"><IconChevronLeft /> Deploy</Link>

      <div className="page-head">
        <div className="cluster" style={{ alignItems: "flex-start" }}>
          <ServerEmojiBadge emoji={serverEmojiOrFallback(server)} size="lg" />
          <div>
            <h1 style={{ margin: 0 }}>{server.name}</h1>
            <p className="subtle" style={{ margin: "4px 0 0" }}>
              {server.os || "unknown"} / {server.arch || "unknown"} ·{" "}
              <span className={`status ${tone[server.status]}`}>{server.status}</span>
              {" · "}{inventory.total} processes
            </p>
          </div>
        </div>
        <div className="page-head-actions">
          <Link href={`/servers/${server.id}`} className="button secondary">Server details</Link>
          <Link href="/deploys/new" className="button"><IconPlus /> New project</Link>
        </div>
      </div>

      <ProcessManagerTabs serverId={server.id} inventory={inventory} />

      {projects.length > 0 && (
        <div style={{ marginTop: 28 }}>
          <div className="card-title" style={{ marginBottom: 12 }}>Projects on this server</div>
          <div className="deploy-project-grid">
            {projects.map((p) => (
              <Link key={p.id} href={`/deploys/${p.id}`} className="deploy-project-card">
                <div className="deploy-project-card-name">{p.name}</div>
                <div className="subtle" style={{ fontSize: 13 }}>
                  {p.provider}/{p.repo_full_name}
                </div>
                <div className="subtle" style={{ fontSize: 12 }}>
                  {p.language || "unknown"} · {p.default_branch}
                </div>
                <div className="deploy-project-card-foot">
                  <span className="pill">{p.process_manager}</span>
                  {p.port > 0 && <span className="pill">PORT {p.port}</span>}
                  <span className="button ghost sm" style={{ marginLeft: "auto" }}>
                    Open
                  </span>
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
