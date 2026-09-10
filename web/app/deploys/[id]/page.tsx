import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { DeployProject, DeployRun, ListResponse, Server } from "@/lib/types";
import CopyButton from "@/components/CopyButton";
import { IconChevronLeft } from "@/components/icons";
import { RedeployButton } from "@/components/deploys/RedeployButton";
import { HostThisApp } from "@/components/deploys/HostThisApp";
import { ProjectActions } from "@/components/deploys/ProjectActions";

type Detail = {
  project: DeployProject;
  webhook_secret?: string;
  webhook_url?: string;
};

const tone: Record<DeployRun["status"], string> = {
  pending: "neutral",
  running: "info",
  succeeded: "ok",
  failed: "danger",
  canceled: "neutral",
  agent_offline: "danger",
};

export default async function DeployDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  let detail: Detail | null = null;
  let runs: DeployRun[] = [];
  let server: Server | null = null;
  let workflow: { croncompose_yml?: string; workflow_yml?: string } | null = null;
  let error: string | null = null;

  try {
    detail = await apiGet<Detail>(`/deploys/${id}`);
    const [runData, wf] = await Promise.all([
      apiGet<ListResponse<DeployRun>>(`/deploys/${id}/runs`),
      apiGet<{ croncompose_yml: string; workflow_yml: string }>(`/deploys/${id}/workflow`).catch(() => null),
    ]);
    runs = runData.items;
    workflow = wf;
    try {
      server = await apiGet<Server>(`/servers/${detail.project.server_id}`);
    } catch { /* ignore */ }
  } catch (e) {
    error = (e as Error).message;
  }

  if (error || !detail) {
    return <div className="form-error">Could not load deploy: <code>{error}</code></div>;
  }

  const p = detail.project;

  return (
    <>
      <Link href="/deploys" className="back-link"><IconChevronLeft /> Back to deploys</Link>
      <div className="page-head">
        <div>
          <h1>{p.name}</h1>
          <p className="subtle">{p.provider}/{p.repo_full_name} · {server?.name || p.server_id.slice(0, 8)}</p>
        </div>
        <div className="page-head-actions">
          <ProjectActions project={p} />
          <RedeployButton projectId={p.id} />
        </div>
      </div>

      <div className="cards">
        <div className="panel">
          <div className="card-title">Clone</div>
          <p className="subtle" style={{ margin: "8px 0 0" }}><code>{p.clone_path}</code></p>
          <div className="cluster" style={{ marginTop: 10 }}>
            <span className="pill">{p.language}</span>
            <span className="pill">{p.default_branch}</span>
            <span className="pill">{p.process_manager}</span>
            {p.port > 0 && <span className="pill">PORT {p.port}</span>}
          </div>
        </div>
        <div className="panel">
          <div className="card-title">Install</div>
          <pre className="subtle" style={{ whiteSpace: "pre-wrap", margin: "8px 0 0" }}>{p.install_script || "(none)"}</pre>
        </div>
      </div>

      {detail.webhook_url && (
        <div className="panel" style={{ marginTop: 18 }}>
          <div className="card-title">Push webhook</div>
          <p className="subtle">Created on the git provider when the git grant allows it. URL and secret if you need to attach it by hand.</p>
          <p><code>{detail.webhook_url}</code> <CopyButton value={detail.webhook_url} /></p>
          {detail.webhook_secret && (
            <p><code>{detail.webhook_secret}</code> <CopyButton value={detail.webhook_secret} label="Copy secret" /></p>
          )}
        </div>
      )}

      <HostThisApp project={p} />

      {workflow?.workflow_yml && (
        <div className="panel" style={{ marginTop: 18 }}>
          <div className="card-title">{p.provider === "gitlab" ? "GitLab CI" : "GitHub Actions"}</div>
          <p className="subtle">Store the deploy token as <code>CRONCOMPOSE_TOKEN</code>. This file is committed on import when the git grant allows it.</p>
          <pre className="term-log" style={{ maxHeight: 240, overflow: "auto" }}>{workflow.workflow_yml}</pre>
        </div>
      )}

      <h2>Runs</h2>
      {runs.length === 0 && <div className="panel"><div className="empty">No runs yet.</div></div>}
      <div className="stack">
        {runs.map((r) => (
          <Link href={`/deploys/runs/${r.id}`} className="panel" key={r.id}>
            <div className="row">
              <div className="cluster">
                <span className={`status ${tone[r.status]}`}>{r.status}</span>
                <span className="pill">{r.trigger}</span>
                <span className="subtle">{r.branch}</span>
              </div>
              <span className="subtle">{r.created_at}</span>
            </div>
          </Link>
        ))}
      </div>
    </>
  );
}
