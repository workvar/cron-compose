import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Me, Passkey, Server, UpdateStatus } from "@/lib/types";
import { AgentRootToggle } from "@/components/AgentRootToggle";
import { JobRow } from "@/components/JobRow";
import { UpdateServerButton } from "@/components/UpdateServerButton";
import { ServerActions } from "@/components/ServerActions";
import { IconChevronLeft, IconPlus, IconTerminal } from "@/components/icons";

const tone: Record<Server["status"], string> = { online: "ok", offline: "danger", pending: "neutral" };

type Props = { params: Promise<{ id: string }> };

export default async function ServerDetailPage({ params }: Props) {
  const { id } = await params;
  let server: Server | null = null;
  let jobs: Job[] = [];
  let me: Me | null = null;
  let updates: UpdateStatus | null = null;
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
  try {
    updates = await apiGet<UpdateStatus>("/updates");
  } catch {
    updates = null;
  }
  const canTerminal = me?.role === "admin" || me?.role === "owner";
  const updateInfo = updates?.items.find((s) => s.server_id === id);
  let hasPasskeys = false;
  if (canTerminal) {
    try {
      hasPasskeys = (await apiGet<ListResponse<Passkey>>("/auth/passkeys")).items.length > 0;
    } catch {
      hasPasskeys = false;
    }
  }

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
          {canTerminal && <ServerActions server={server} />}
        </div>
      </div>

      {server.description && <p className="subtle" style={{ marginTop: -8, marginBottom: 18 }}>{server.description}</p>}

      {canTerminal && <AgentRootToggle server={server} hasPasskeys={hasPasskeys} />}

      {canTerminal && updateInfo && (
        <UpdateServerButton
          serverId={server.id}
          currentVersion={updateInfo.current_version}
          targetVersion={updates?.latest_version}
          canUpdate={updateInfo.can_update}
          updateAvailable={updateInfo.update_available}
          stack={!!updateInfo.stack}
        />
      )}

      <h2>Jobs</h2>
      {jobs.length === 0 ? (
        <div className="panel"><div className="empty">No jobs yet on this server.</div></div>
      ) : (
        <div className="stack">{jobs.map((j) => <JobRow key={j.id} job={j} />)}</div>
      )}
    </>
  );
}
