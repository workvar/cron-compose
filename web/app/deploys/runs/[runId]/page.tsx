"use client";

import { useEffect, useState, use } from "react";
import Link from "next/link";
import type { DeployProject, DeployRun, LogLine } from "@/lib/types";
import { IconChevronLeft } from "@/components/icons";
import { TerminalFrame } from "@/components/terminal/TerminalFrame";
import { HostThisApp } from "@/components/deploys/HostThisApp";

type Props = { params: Promise<{ runId: string }> };

const tone: Record<DeployRun["status"], string> = {
  pending: "neutral",
  running: "info",
  succeeded: "ok",
  failed: "danger",
  canceled: "neutral",
  agent_offline: "danger",
};

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

  useEffect(() => {
    const es = new EventSource(`/api/deploy-runs/${runId}/logs/stream`);
    es.addEventListener("log", (ev) => {
      try {
        const line = JSON.parse((ev as MessageEvent).data) as LogLine;
        setLogs((prev) => [...prev, line]);
      } catch { /* ignore */ }
    });
    es.addEventListener("done", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status: DeployRun["status"]; exit_code?: number };
        setRun((prev) => (prev ? { ...prev, status: data.status, exit_code: data.exit_code } : prev));
      } catch { /* ignore */ }
      es.close();
    });
    es.onerror = () => es.close();
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
            {run.exit_code !== undefined && <span className="pill">exit {run.exit_code}</span>}
          </div>
        </div>
      </div>

      {run.error && <p className="form-error">{run.error}</p>}

      <h2>Logs <span className="subtle" style={{ fontSize: 13, fontWeight: 500 }}>· live</span></h2>
      <TerminalFrame title={`${run.trigger} ${run.id.slice(0, 8)} · ${run.status}`}>
        <pre className="term-log">
          {logs.length === 0 ? (
            <span className="term-log-empty">(no output yet)</span>
          ) : (
            logs.map((l, i) => <span key={`${l.seq}-${i}`}>{l.chunk}</span>)
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
