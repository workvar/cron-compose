"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { BrandLoader } from "@/components/BrandLoader";
import { AreaChart } from "@/components/charts/AreaChart";
import { BarChart } from "@/components/charts/BarChart";
import {
  formatBytes,
  formatDurationMs,
  phaseLabel,
  type DeployAnalytics,
} from "@/lib/deploy-analytics";

type Props = {
  /** Project id for per-project charts, or omit for fleet-wide. */
  projectId?: string;
  title?: string;
  compact?: boolean;
};

export function DeployAnalyticsPanel({ projectId, title, compact }: Props) {
  const [data, setData] = useState<DeployAnalytics | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    const path = projectId
      ? `/api/deploys/${encodeURIComponent(projectId)}/analytics`
      : "/api/deploys/analytics";
    fetch(path)
      .then(async (r) => {
        if (!r.ok) throw new Error(await r.text());
        return r.json() as Promise<DeployAnalytics>;
      })
      .then((d) => {
        if (live) setData(d);
      })
      .catch((e) => {
        if (live) setError((e as Error).message);
      });
    return () => {
      live = false;
    };
  }, [projectId]);

  if (error) {
    return (
      <div className="panel" style={{ marginTop: compact ? 0 : 18 }}>
        <div className="card-title">{title || "Deploy analytics"}</div>
        <p className="form-error">Could not load analytics: <code>{error}</code></p>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="panel" style={{ marginTop: compact ? 0 : 18 }}>
        <div className="card-title">{title || "Deploy analytics"}</div>
        <BrandLoader inline size={28} caption="Loading analytics…" />
      </div>
    );
  }

  if (data.runs_total === 0) {
    return (
      <div className="panel" style={{ marginTop: compact ? 0 : 18 }}>
        <div className="card-title">{title || "Deploy analytics"}</div>
        <p className="subtle" style={{ marginTop: 8 }}>
          No deploy runs yet. After a few deploys you will see compile times, release size, and phase
          breakdowns here.
        </p>
      </div>
    );
  }

  const durationSeries = data.duration_trend.map((p) => ({
    label: p.label,
    value: Math.round(p.value / 1000),
  }));
  const releaseSeries = data.release_trend.map((p) => ({
    label: p.label,
    value: Math.round(p.value / (1024 * 1024)),
  }));
  const phaseBars = data.phase_avg_ms.map((p) => ({
    label: phaseLabel(p.phase),
    value: Math.round(p.ms / 1000),
  }));

  return (
    <div className="deploy-analytics" style={{ marginTop: compact ? 0 : 18 }}>
      <div className="page-head" style={{ marginBottom: 14 }}>
        <div>
          <h2 style={{ margin: 0, fontSize: 18 }}>{title || "Deploy analytics"}</h2>
          <p className="subtle" style={{ margin: "4px 0 0" }}>
            Build time, release size, and phase breakdown from recent runs
            {data.avg_release_bytes > 0 ? "" : " (size appears after agents report metrics)"}.
          </p>
        </div>
      </div>

      <div className="dash-grid deploy-analytics-grid">
        <div className="panel">
          <div className="card-title">Success</div>
          <p style={{ fontSize: 28, fontWeight: 800, margin: "8px 0 0" }}>{data.success_rate}%</p>
          <p className="subtle" style={{ fontSize: 13 }}>
            {data.succeeded} ok · {data.failed} failed · {data.runs_total} sampled
          </p>
        </div>
        <div className="panel">
          <div className="card-title">Avg deploy time</div>
          <p style={{ fontSize: 28, fontWeight: 800, margin: "8px 0 0" }}>
            {formatDurationMs(data.avg_duration_ms)}
          </p>
          <p className="subtle" style={{ fontSize: 13 }}>End-to-end clone → health</p>
        </div>
        <div className="panel">
          <div className="card-title">Avg release size</div>
          <p style={{ fontSize: 28, fontWeight: 800, margin: "8px 0 0" }}>
            {formatBytes(data.avg_release_bytes)}
          </p>
          <p className="subtle" style={{ fontSize: 13 }}>On disk after install / cleanup</p>
        </div>

        {durationSeries.length > 0 && (
          <section className="panel chart-card span-2">
            <div className="card-head">
              <div>
                <div className="card-title">Deploy duration trend</div>
                <div className="subtle" style={{ fontSize: 12 }}>Seconds per run (oldest → newest)</div>
              </div>
            </div>
            <AreaChart data={durationSeries} />
          </section>
        )}

        {phaseBars.length > 0 && (
          <section className="panel chart-card span-2">
            <div className="card-head">
              <div>
                <div className="card-title">Time by phase</div>
                <div className="subtle" style={{ fontSize: 12 }}>
                  Average seconds — build (install) is usually the compile step
                </div>
              </div>
            </div>
            <BarChart data={phaseBars} />
          </section>
        )}

        {releaseSeries.length > 0 && (
          <section className="panel chart-card span-2">
            <div className="card-head">
              <div>
                <div className="card-title">Release size trend</div>
                <div className="subtle" style={{ fontSize: 12 }}>Megabytes on disk after each deploy</div>
              </div>
            </div>
            <AreaChart data={releaseSeries} />
          </section>
        )}

        {!projectId && data.by_project && data.by_project.length > 0 && (
          <section className="panel span-2">
            <div className="card-head">
              <div className="card-title">By project</div>
            </div>
            <div className="mini-list">
              {data.by_project.slice(0, 8).map((p) => (
                <Link
                  key={p.project_id}
                  href={`/deploys/${p.project_id}`}
                  className="mini-row"
                  style={{ color: "inherit" }}
                >
                  <span className="mini-body">
                    <span className="mini-title">{p.name || p.project_id.slice(0, 8)}</span>
                    <span className="mini-sub">
                      {p.runs} runs · {p.success_rate}% ok · avg {formatDurationMs(p.avg_duration_ms)}
                      {p.avg_release_bytes > 0 ? ` · ${formatBytes(p.avg_release_bytes)}` : ""}
                    </span>
                  </span>
                </Link>
              ))}
            </div>
          </section>
        )}

        {data.recent.length > 0 && (
          <section className="panel span-2">
            <div className="card-head">
              <div className="card-title">Recent runs</div>
            </div>
            <div className="mini-list">
              {data.recent.map((r) => (
                <Link
                  key={r.id}
                  href={`/deploys/runs/${r.id}`}
                  className="mini-row"
                  style={{ color: "inherit" }}
                >
                  <span className="mini-body">
                    <span className="mini-title">
                      {r.project_name ? `${r.project_name} · ` : ""}
                      {r.branch}
                      {r.commit_sha ? ` @ ${r.commit_sha.slice(0, 7)}` : ""}
                    </span>
                    <span className="mini-sub">
                      {formatDurationMs(r.duration_ms)}
                      {r.release_bytes ? ` · ${formatBytes(r.release_bytes)}` : ""}
                      {r.phase_ms?.install
                        ? ` · build ${formatDurationMs(r.phase_ms.install)}`
                        : ""}
                    </span>
                  </span>
                  <span className={`status ${r.status === "succeeded" ? "ok" : r.status === "failed" ? "danger" : "neutral"}`}>
                    {r.status}
                  </span>
                </Link>
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
