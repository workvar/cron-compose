"use client";

import { useCallback, useEffect, useState, use } from "react";
import Link from "next/link";
import type { DeployProject, DeployRun } from "@/lib/types";
import { useDeployRunLogs, type DeployDone } from "@/lib/useDeployRunLogs";
import { BrandLoaderBlock } from "@/components/BrandLoader";
import { IconChevronLeft } from "@/components/icons";
import { HostThisApp } from "@/components/deploys/HostThisApp";
import { DeployRunBoard } from "@/components/deploys/DeployRunBoard";
import { DeployResultBanner } from "@/components/deploys/DeployResultBanner";

type Props = { params: Promise<{ runId: string }> };

const tone: Record<DeployRun["status"], string> = {
  pending: "neutral",
  running: "info",
  succeeded: "ok",
  failed: "danger",
  canceled: "neutral",
  agent_offline: "danger",
};

const terminalStatus = new Set<DeployRun["status"]>([
  "succeeded",
  "failed",
  "canceled",
  "agent_offline",
]);

export default function DeployRunPage({ params }: Props) {
  const { runId } = use(params);
  const [run, setRun] = useState<DeployRun | null>(null);
  const [project, setProject] = useState<DeployProject | null>(null);
  const [stdin, setStdin] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch(`/api/deploy-runs/${runId}`)
      .then((r) => r.json() as Promise<DeployRun>)
      .then((r) => { if (!cancelled) setRun(r); })
      .catch((e) => { if (!cancelled) setError((e as Error).message); });
    return () => { cancelled = true; };
  }, [runId]);

  useEffect(() => {
    if (!run?.project_id) return;
    let cancelled = false;
    fetch(`/api/deploys/${run.project_id}`)
      .then((r) => r.json() as Promise<{ project: DeployProject }>)
      .then((d) => { if (!cancelled) setProject(d.project); })
      .catch(() => { /* ignore */ });
    return () => { cancelled = true; };
  }, [run?.project_id]);

  const refreshRun = useCallback(() => {
    fetch(`/api/deploy-runs/${runId}`)
      .then((r) => r.json() as Promise<DeployRun>)
      .then((r) => setRun(r))
      .catch(() => { /* ignore */ });
  }, [runId]);

  const onDone = useCallback((data: DeployDone) => {
    if (data.status) {
      const status = data.status as DeployRun["status"];
      setRun((prev) => (prev ? {
        ...prev,
        status,
        exit_code: data.exit_code,
        // Success messages must not land in error (older agents sent "deploy finished").
        error: status === "succeeded" ? undefined : (data.error || prev.error),
      } : prev));
    }
    // Refetch so error/exit_code match the DB even if the done payload was sparse.
    refreshRun();
  }, [refreshRun]);

  const live = !run || run.status === "pending" || run.status === "running";
  const logs = useDeployRunLogs(runId, live, onDone);

  // Status safety net: if the live "done" event never arrives (proxy cut, tab
  // asleep), the page still notices the run finished.
  useEffect(() => {
    if (!live) return;
    const id = window.setInterval(refreshRun, 5000);
    return () => window.clearInterval(id);
  }, [live, refreshRun]);

  async function cancelDeploy() {
    const ok = window.confirm("Cancel this deploy? The command running on the server will be stopped.");
    if (!ok) return;
    const res = await fetch(`/api/deploy-runs/${runId}/cancel`, { method: "POST" });
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
      setError(body?.error?.message || "Could not cancel the deploy");
    }
  }

  async function sendStdin(e: React.FormEvent) {
    e.preventDefault();
    if (!stdin) return;
    await fetch(`/api/deploy-runs/${runId}/stdin`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ data: stdin.endsWith("\n") ? stdin : stdin + "\n" }),
    });
    setStdin("");
  }

  if (error && !run) {
    return <div className="form-error">Could not load run: <code>{error}</code></div>;
  }
  if (!run) return <BrandLoaderBlock caption="Loading deploy…" />;

  const showExit = run.exit_code !== undefined && terminalStatus.has(run.status);

  return (
    <>
      <Link href={`/deploys/${run.project_id}`} className="back-link"><IconChevronLeft /> Back to project</Link>
      <div className="page-head">
        <div>
          <h1>Deploy {run.id.slice(0, 8)}</h1>
          <div className="cluster" style={{ marginTop: 6, gap: 8 }}>
            <span className={`status ${tone[run.status]}`}>{run.status}</span>
            <span className="pill">{run.trigger}</span>
            <span className="pill">{run.branch}</span>
            {showExit && <span className="pill">exit {run.exit_code}</span>}
            {live && (
              <button type="button" className="button danger sm" onClick={() => void cancelDeploy()}>
                Cancel deploy
              </button>
            )}
          </div>
        </div>
      </div>

      <DeployResultBanner run={run} project={project} />

      <DeployRunBoard logs={logs} project={project} status={run.status} live={live} runId={run.id} />

      {live && (
        <form onSubmit={sendStdin} className="cluster" style={{ marginTop: 12 }}>
          <input
            value={stdin}
            onChange={(e) => setStdin(e.target.value)}
            placeholder="Installer prompt (sent to stdin)"
            style={{ flex: 1 }}
          />
          <button type="submit" className="button sm">Send</button>
        </form>
      )}

      {run.status === "succeeded" && project && <HostThisApp project={project} />}
    </>
  );
}
