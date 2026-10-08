"use client";

import { useEffect, useState, use } from "react";
import Link from "next/link";
import type { DeployProject, DeployRun, LogLine } from "@/lib/types";
import { IconChevronLeft } from "@/components/icons";
import { TerminalFrame } from "@/components/terminal/TerminalFrame";
import { HostThisApp } from "@/components/deploys/HostThisApp";
import { stripAnsi } from "@/lib/strip-ansi";

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

function mergeLogs(prev: LogLine[], next: LogLine[]): LogLine[] {
  if (next.length === 0) return prev;
  const seen = new Set(prev.map((l) => `${l.stream}:${l.seq}`));
  const extra = next.filter((l) => !seen.has(`${l.stream}:${l.seq}`));
  if (extra.length === 0) return prev;
  return [...prev, ...extra].sort((a, b) => a.seq - b.seq);
}

export default function DeployRunPage({ params }: Props) {
  const { runId } = use(params);
  const [run, setRun] = useState<DeployRun | null>(null);
  const [project, setProject] = useState<DeployProject | null>(null);
  const [logs, setLogs] = useState<LogLine[]>([]);
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

  // Snapshot fetch: works even when EventSource fails (proxy buffering, brief blip).
  useEffect(() => {
    let cancelled = false;
    const load = () => {
      fetch(`/api/deploy-runs/${runId}/logs`)
        .then((r) => r.json() as Promise<{ items: LogLine[] }>)
        .then((d) => {
          if (!cancelled && Array.isArray(d.items)) {
            setLogs((prev) => mergeLogs(prev, d.items));
          }
        })
        .catch(() => { /* ignore */ });
    };
    load();
    const live = !run || run.status === "pending" || run.status === "running";
    if (!live) return () => { cancelled = true; };
    const id = window.setInterval(load, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [runId, run?.status]);

  useEffect(() => {
    const es = new EventSource(`/api/deploy-runs/${runId}/logs/stream`);
    es.addEventListener("log", (ev) => {
      try {
        const line = JSON.parse((ev as MessageEvent).data) as LogLine;
        setLogs((prev) => mergeLogs(prev, [line]));
      } catch { /* ignore */ }
    });
    es.addEventListener("done", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as {
          status: DeployRun["status"];
          exit_code?: number;
          error?: string;
        };
        setRun((prev) => (prev ? {
          ...prev,
          status: data.status,
          exit_code: data.exit_code,
          error: data.error || prev.error,
        } : prev));
      } catch { /* ignore */ }
      // Refetch so error/exit_code match the DB even if the done payload was sparse.
      fetch(`/api/deploy-runs/${runId}`)
        .then((r) => r.json() as Promise<DeployRun>)
        .then((r) => setRun(r))
        .catch(() => { /* ignore */ });
      es.close();
    });
    // Do not close permanently on the first error — EventSource reconnects by default
    // unless we close it. Closing here left the page stuck on "(no output yet)".
    es.onerror = () => { /* allow browser reconnect */ };
    return () => es.close();
  }, [runId]);

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
  if (!run) return <p className="subtle">Loading…</p>;

  const live = run.status === "pending" || run.status === "running";
  const showExit = run.exit_code !== undefined && terminalStatus.has(run.status);

  return (
    <>
      <Link href={`/deploys/${run.project_id}`} className="back-link"><IconChevronLeft /> Back to project</Link>
      <div className="page-head">
        <div>
          <h1>Deploy {run.id.slice(0, 8)}</h1>
          <div className="cluster" style={{ marginTop: 6 }}>
            <span className={`status ${tone[run.status]}`}>{run.status}</span>
            <span className="pill">{run.trigger}</span>
            <span className="pill">{run.branch}</span>
            {showExit && <span className="pill">exit {run.exit_code}</span>}
          </div>
        </div>
      </div>

      {run.error && <p className="form-error">{run.error}</p>}

      <h2>Logs <span className="subtle" style={{ fontSize: 13, fontWeight: 500 }}>· live</span></h2>
      <TerminalFrame title={`${run.trigger} ${run.id.slice(0, 8)} · ${run.status}`}>
        <pre className="term-log">
          {logs.length === 0 ? (
            <span className="term-log-empty">
              {live ? "(waiting for agent output…)" : "(no output)"}
            </span>
          ) : (
            logs.map((l, i) => <span key={`${l.seq}-${i}`}>{stripAnsi(l.chunk)}</span>)
          )}
        </pre>
      </TerminalFrame>

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
