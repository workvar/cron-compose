"use client";

import type { DeployProject, DeployRun } from "@/lib/types";
import { deployDurationMs, formatDeployDuration, shortCommit } from "@/lib/deploy-duration";

type Props = {
  run: DeployRun;
  project: DeployProject | null;
};

const TITLES: Record<DeployRun["status"], string> = {
  pending: "Deploy queued",
  running: "Deploy in progress",
  succeeded: "Deploy succeeded",
  failed: "Deploy failed",
  canceled: "Deploy canceled",
  agent_offline: "Agent offline",
};

function appList(project: DeployProject | null): string {
  if (!project) return "";
  const apps = project.apps || [];
  if (apps.length === 0) return project.name || "";
  return apps.map((a) => a.name || a.root || "app").join(", ");
}

/**
 * Summary card shown when a deploy run reaches a terminal state. Replaces the old
 * red form-error that wrongly showed the success message "deploy finished".
 */
export function DeployResultBanner({ run, project }: Props) {
  const terminal = run.status !== "pending" && run.status !== "running";
  if (!terminal) return null;

  const ok = run.status === "succeeded";
  const ms = deployDurationMs(run.started_at, run.finished_at);
  const commit = shortCommit(run.commit_sha);
  const apps = appList(project);
  // Ignore stale success messages that older agents/control planes stored in error.
  const errText =
    !ok && run.error && !/^deploy finished$/i.test(run.error.trim()) ? run.error.trim() : "";

  const stats: { label: string; value: string }[] = [];
  if (project?.name) stats.push({ label: "Project", value: project.name });
  if (apps && apps !== project?.name) stats.push({ label: "Apps", value: apps });
  if (run.branch) stats.push({ label: "Branch", value: run.branch });
  if (commit) stats.push({ label: "Commit", value: commit });
  if (run.trigger) stats.push({ label: "Trigger", value: run.trigger });
  if (ms != null) stats.push({ label: "Duration", value: formatDeployDuration(ms) });
  if (run.exit_code !== undefined && run.exit_code !== null && !ok) {
    stats.push({ label: "Exit", value: String(run.exit_code) });
  }
  if (project?.clone_path) stats.push({ label: "Path", value: project.clone_path });

  return (
    <div
      className={`deploy-result-banner ${ok ? "is-ok" : "is-bad"}`}
      role={ok ? "status" : "alert"}
    >
      <div className="deploy-result-banner-head">
        <span className="deploy-result-banner-dot" aria-hidden="true" />
        <div>
          <strong>{TITLES[run.status] || run.status}</strong>
          {ok ? (
            <p>
              {project?.name ? `${project.name} is live` : "Release activated"}
              {ms != null ? ` · took ${formatDeployDuration(ms)}` : ""}
              {commit ? ` · ${commit}` : ""}
            </p>
          ) : (
            <p>
              {errText || "The deploy did not complete successfully."}
              {ms != null ? ` · ran for ${formatDeployDuration(ms)}` : ""}
            </p>
          )}
        </div>
      </div>
      {stats.length > 0 && (
        <dl className="deploy-result-stats">
          {stats.map((s) => (
            <div key={s.label} className="deploy-result-stat">
              <dt>{s.label}</dt>
              <dd>
                <code>{s.value}</code>
              </dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}
