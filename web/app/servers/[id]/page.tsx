import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Me, Server } from "@/lib/types";
import { JobRow } from "@/components/JobRow";
import { IconChevronLeft, IconPlus, IconTerminal } from "@/components/icons";

const tone: Record<Server["status"], string> = { online: "ok", offline: "danger", pending: "neutral" };

type Props = { params: Promise<{ id: string }> };

export default async function ServerDetailPage({ params }: Props) {
  const { id } = await params;
  let server: Server | null = null;
  let jobs: Job[] = [];
  let me: Me | null = null;
  let error: string | null = null;
  try {
    server = await apiGet<Server>(`/servers/${id}`);
    jobs = (await apiGet<ListResponse<Job>>(`/jobs?server=${id}`)).items;
  } catch (e) {
    error = (e as Error).message;
  }
  try {
    me = await apiGet<Me>("/me");
  } catch {
    me = null;
  }
  const canTerminal = me?.role === "admin" || me?.role === "owner";

  if (error || !server) {
    return (
      <>
        <Link href="/servers" className="back-link"><IconChevronLeft /> Servers</Link>
        <div className="form-error">Could not load server: <code>{error ?? "not found"}</code></div>
      </>
    );
  }

  return (
    <>
      <Link href="/servers" className="back-link"><IconChevronLeft /> Servers</Link>
      <div className="page-head">
        <div>
          <h1>{server.name}</h1>
          <div className="cluster" style={{ marginTop: 6 }}>
            <span className={`status ${tone[server.status]}`}>{server.status}</span>
            <span className="pill">{server.os || "unknown"} / {server.arch || "unknown"}</span>
            {server.agent_version && <span className="pill">agent {server.agent_version}</span>}
            {server.last_seen_at && <span className="pill">seen {new Date(server.last_seen_at).toLocaleString()}</span>}
          </div>
        </div>
        <div className="page-head-actions">
          {canTerminal && (
            <Link href={`/servers/${server.id}/terminal`} className="button secondary"><IconTerminal /> Terminal</Link>
          )}
          <Link href={`/servers/${server.id}/jobs/new`} className="button"><IconPlus /> New job</Link>
        </div>
      </div>

      {server.description && <p className="subtle" style={{ marginTop: -8, marginBottom: 18 }}>{server.description}</p>}

      <h2>Jobs</h2>
      {jobs.length === 0 ? (
        <div className="panel"><div className="empty">No jobs yet on this server.</div></div>
      ) : (
        <div className="stack">{jobs.map((j) => <JobRow key={j.id} job={j} />)}</div>
      )}
    </>
  );
}
