import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Server } from "@/lib/types";
import { IconChevronLeft, IconPlus } from "@/components/icons";
import { ServerEmojiBadge } from "@/components/ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";
import { JobCard } from "@/components/jobs/JobCard";

const tone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

type Props = { params: Promise<{ id: string }> };

export default async function JobsServerPage({ params }: Props) {
  const { id } = await params;
  let server: Server | null = null;
  let jobs: Job[] = [];
  let error: string | null = null;

  try {
    server = await apiGet<Server>(`/servers/${id}`);
    jobs = (await apiGet<ListResponse<Job>>(`/jobs?server=${id}`)).items;
  } catch (e) {
    error = (e as Error).message;
  }

  if (error || !server) {
    return (
      <>
        <Link href="/jobs" className="back-link"><IconChevronLeft /> Jobs</Link>
        <div className="form-error">Could not load server: <code>{error ?? "not found"}</code></div>
      </>
    );
  }

  const enabled = jobs.filter((j) => j.enabled).length;

  return (
    <>
      <Link href="/jobs" className="back-link"><IconChevronLeft /> Jobs</Link>

      <div className="page-head">
        <div className="cluster" style={{ alignItems: "flex-start" }}>
          <ServerEmojiBadge emoji={serverEmojiOrFallback(server)} size="lg" />
          <div>
            <h1 style={{ margin: 0 }}>{server.name}</h1>
            <p className="subtle" style={{ margin: "4px 0 0" }}>
              {server.os || "unknown"} / {server.arch || "unknown"} ·{" "}
              <span className={`status ${tone[server.status]}`}>{server.status}</span>
              {" · "}{jobs.length} {jobs.length === 1 ? "job" : "jobs"}
              {jobs.length > 0 ? ` · ${enabled} enabled` : null}
            </p>
          </div>
        </div>
        <div className="page-head-actions">
          <Link href={`/servers/${server.id}`} className="button secondary">Server details</Link>
          <Link href={`/servers/${server.id}/jobs/new`} className="button"><IconPlus /> New job</Link>
        </div>
      </div>

      {jobs.length === 0 ? (
        <div className="panel">
          <div className="empty">
            No jobs yet on this server.{" "}
            <Link href={`/servers/${server.id}/jobs/new`}>Create your first job</Link>.
          </div>
        </div>
      ) : (
        <div className="deploy-project-grid">
          {jobs.map((j) => <JobCard key={j.id} job={j} />)}
        </div>
      )}
    </>
  );
}
