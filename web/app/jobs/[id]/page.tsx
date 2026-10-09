import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Run, Server } from "@/lib/types";
import { RunRow } from "@/components/RunRow";
import { RunNowButton } from "./RunNowButton";
import { IconChevronLeft } from "@/components/icons";
import { DeployServerChip } from "@/components/deploys/DeployServerChip";

type Props = { params: Promise<{ id: string }> };

function targetLabel(job: Job): string {
  if (job.target_kind === "labels") {
    const pairs = Object.entries(job.target_labels || {}).map(([k, v]) => `${k}=${v}`).join(", ");
    return pairs || "label selector";
  }
  return "single server";
}

export default async function JobDetailPage({ params }: Props) {
  const { id } = await params;
  let job: Job | null = null;
  let runs: Run[] = [];
  let server: Server | null = null;
  let error: string | null = null;
  try {
    job = await apiGet<Job>(`/jobs/${id}`);
    runs = (await apiGet<ListResponse<Run>>(`/jobs/${id}/runs?limit=20`)).items;
    if (job.server_id) {
      try {
        server = await apiGet<Server>(`/servers/${job.server_id}`);
      } catch { /* ignore */ }
    }
  } catch (e) {
    error = (e as Error).message;
  }

  if (error || !job) {
    return <div className="form-error">Could not load job: <code>{error ?? "not found"}</code></div>;
  }

  const backHref = job.server_id ? `/jobs/servers/${job.server_id}` : "/jobs";

  return (
    <>
      <Link href={backHref} className="back-link"><IconChevronLeft /> Back to jobs</Link>
      <div className="page-head">
        <div>
          <div className="cluster" style={{ alignItems: "center", gap: 10 }}>
            <h1 style={{ margin: 0 }}>{job.name}</h1>
            <span className={`status ${job.enabled ? "ok" : "neutral"}`}>
              {job.enabled ? "enabled" : "disabled"}
            </span>
          </div>
          {job.description ? (
            <p className="subtle">{job.description}</p>
          ) : (
            <p className="subtle">
              <code>{job.schedule_cron}</code> · {job.timezone}
            </p>
          )}
        </div>
        <div className="page-head-actions">
          <RunNowButton jobId={job.id} />
        </div>
      </div>

      <div className="deploy-summary">
        <div className="panel deploy-summary-main">
          <div className="deploy-summary-meta">
            {server ? (
              <DeployServerChip server={server} label="Runs on" />
            ) : (
              <span className="subtle">{targetLabel(job)}</span>
            )}
            <div className="cluster">
              <span className="pill"><code>{job.schedule_cron}</code></span>
              <span className="pill">{job.timezone}</span>
              <span className="pill">{job.interpreter}</span>
              <span className="pill">v{job.current_version}</span>
              <span className="pill">{job.concurrency_policy}</span>
              {job.timeout_seconds > 0 && (
                <span className="pill">{job.timeout_seconds}s timeout</span>
              )}
              {job.run_as_user ? (
                <span className="pill" title="OS account for the run">as {job.run_as_user}</span>
              ) : null}
            </div>
          </div>
          <div className="deploy-summary-grid">
            <div>
              <div className="card-title">Target</div>
              <p className="subtle" style={{ margin: "6px 0 0" }}>{targetLabel(job)}</p>
            </div>
            <div>
              <div className="card-title">Working dir</div>
              <p className="subtle" style={{ margin: "6px 0 0" }}>
                <code>{job.working_dir || "(default)"}</code>
              </p>
            </div>
          </div>
        </div>
        <div className="panel deploy-summary-redeploy">
          <div className="card-title">Run now</div>
          <p className="subtle" style={{ margin: "6px 0 12px" }}>
            Fire this job on the agent immediately, outside the cron schedule.
          </p>
          <RunNowButton jobId={job.id} />
        </div>
      </div>

      <h2>Script</h2>
      <pre className="review-script" style={{ maxHeight: 420 }}>{job.script_body}</pre>

      <h2>Recent runs</h2>
      {runs.length === 0 ? (
        <div className="panel"><div className="empty">No runs yet.</div></div>
      ) : (
        <div className="stack">{runs.map((r) => <RunRow key={r.id} run={r} />)}</div>
      )}
    </>
  );
}
