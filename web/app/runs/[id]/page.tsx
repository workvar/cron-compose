"use client";

import { useEffect, useState, use } from "react";
import Link from "next/link";
import type { LogLine, Run } from "@/lib/types";
import { IconChevronLeft } from "@/components/icons";
import { TerminalFrame } from "@/components/terminal/TerminalFrame";

type Props = { params: Promise<{ id: string }> };

const tone: Record<Run["status"], string> = {
  pending: "neutral",
  running: "info",
  succeeded: "ok",
  failed: "danger",
  timed_out: "danger",
  canceled: "neutral",
  skipped: "neutral",
};

export default function RunDetailPage({ params }: Props) {
  const { id } = use(params);
  const [run, setRun] = useState<Run | null>(null);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch(`/api/runs/${id}`)
      .then((r) => r.json() as Promise<Run>)
      .then((r) => { if (!cancelled) setRun(r); })
      .catch((e) => { if (!cancelled) setError((e as Error).message); });
    return () => { cancelled = true; };
  }, [id]);

  useEffect(() => {
    const es = new EventSource(`/api/runs/${id}/logs/stream`);
    es.addEventListener("log", (ev) => {
      try {
        const line = JSON.parse((ev as MessageEvent).data) as LogLine;
        setLogs((prev) => [...prev, line]);
      } catch { /* ignore malformed */ }
    });
    es.addEventListener("done", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status: Run["status"]; exit_code?: number };
        setRun((prev) => (prev ? { ...prev, status: data.status, exit_code: data.exit_code } : prev));
      } catch { /* ignore */ }
      es.close();
    });
    es.onerror = () => es.close();
    return () => es.close();
  }, [id]);

  if (error && !run) {
    return <div className="form-error">Could not load run: <code>{error}</code></div>;
  }
  if (!run) return <p className="subtle">Loading…</p>;

  return (
    <>
      <Link href={`/jobs/${run.job_id}`} className="back-link"><IconChevronLeft /> Back to job</Link>
      <div className="page-head">
        <div>
          <h1>Run {run.id.slice(0, 8)}</h1>
          <div className="cluster" style={{ marginTop: 6 }}>
            <span className={`status ${tone[run.status]}`}>{run.status}</span>
            <span className="pill">{run.trigger}</span>
            {run.exit_code !== undefined && <span className="pill">exit {run.exit_code}</span>}
            {run.duration_ms !== undefined && <span className="pill">{run.duration_ms} ms</span>}
          </div>
        </div>
      </div>

      <h2>Logs <span className="subtle" style={{ fontSize: 13, fontWeight: 500 }}>· live</span></h2>
      <TerminalFrame title={`${run.trigger} run ${run.id.slice(0, 8)} · ${run.status}`}>
        <pre className="term-log">
          {logs.length === 0 ? (
            <span className="term-log-empty">(no output yet)</span>
          ) : (
            logs.map((l) => (
              <div key={`${l.stream}-${l.seq}`} className={l.stream === "stderr" ? "term-stderr" : "term-stdout"}>
                <span className="term-stream">{l.stream}</span>
                {l.chunk}
              </div>
            ))
          )}
        </pre>
      </TerminalFrame>
    </>
  );
}
