import Link from "next/link";
import type { Job } from "@/lib/types";

function targetSummary(job: Job): string {
  if (job.target_kind === "labels") {
    const pairs = Object.entries(job.target_labels || {}).map(([k, v]) => `${k}=${v}`).join(", ");
    return pairs ? `labels · ${pairs}` : "label selector";
  }
  return "this server";
}

/** Project-style card used on Jobs server pages (matches deploy project cards). */
export function JobCard({ job }: { job: Job }) {
  return (
    <Link href={`/jobs/${job.id}`} className="deploy-project-card">
      <div className="deploy-project-card-name">{job.name}</div>
      {job.description ? (
        <div className="subtle" style={{ fontSize: 13 }}>{job.description}</div>
      ) : null}
      <div className="subtle" style={{ fontSize: 12 }}>
        <code>{job.schedule_cron}</code> · {job.timezone}
      </div>
      <div className="deploy-project-card-foot">
        <span className={`status ${job.enabled ? "ok" : "neutral"}`}>
          {job.enabled ? "enabled" : "disabled"}
        </span>
        <span className="pill">{job.interpreter}</span>
        <span className="pill">v{job.current_version}</span>
        <span className="pill">{targetSummary(job)}</span>
        <span className="button ghost sm" style={{ marginLeft: "auto" }}>
          Open
        </span>
      </div>
    </Link>
  );
}
