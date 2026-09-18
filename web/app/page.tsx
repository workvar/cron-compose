import Link from "next/link";
import { getDashboardData } from "@/lib/dashboard";
import { StatCard } from "@/components/StatCard";
import { UpdateBanner } from "@/components/UpdateBanner";
import { BarChart } from "@/components/charts/BarChart";
import { Gauge } from "@/components/charts/Gauge";
import { AreaChart } from "@/components/charts/AreaChart";
import { Heatmap } from "@/components/charts/Heatmap";
import { ScatterPlot } from "@/components/charts/ScatterPlot";
import { BubbleChart } from "@/components/charts/BubbleChart";
import { RadarChart } from "@/components/charts/RadarChart";
import { Treemap } from "@/components/charts/Treemap";
import { IconPlus, IconJobs, IconServer, IconChevronRight } from "@/components/icons";
import type { Server } from "@/lib/types";

const serverTone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

function targetSummary(job: { target_kind: string; target_labels: Record<string, string> }): string {
  if (job.target_kind === "labels") {
    const pairs = Object.entries(job.target_labels).map(([k, v]) => `${k}=${v}`).join(", ");
    return pairs ? `labels: ${pairs}` : "label selector";
  }
  return "single server";
}

function formatBytes(n: number): string {
  if (!n || n <= 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatUptime(sec?: number): string {
  if (!sec || sec <= 0) return "—";
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  if (d > 0) return `${d}d ${h}h`;
  const m = Math.floor((sec % 3600) / 60);
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

export default async function DashboardPage() {
  const d = await getDashboardData();
  const newJobHref = d.servers.length > 0 ? `/servers/${d.servers[0].id}/jobs/new` : "/servers/new";
  const host = d.host;

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Dashboard</h1>
          <p className="subtle">Schedule, run, and watch jobs across your Linux fleet.</p>
        </div>
        <div className="page-head-actions">
          <Link href="/servers/new" className="button"><IconPlus /> Add server</Link>
          <Link href="/jobs" className="button secondary">View jobs</Link>
        </div>
      </div>

      {!d.reachable && (
        <div className="form-error" style={{ marginBottom: 20 }}>
          Could not reach the control plane. Showing what we have.
        </div>
      )}

      {d.updates && <UpdateBanner status={d.updates} />}

      <div className="dash-grid">
        {/* Row 1 — stat cards */}
        <StatCard
          green
          label="Servers"
          value={d.serverCounts.total}
          href="/servers"
          foot="In your fleet"
          chip={{ text: `${d.serverCounts.online} online`, tone: "up" }}
        />
        <StatCard
          label="Jobs"
          value={d.jobCounts.total}
          href="/jobs"
          foot="Scheduled definitions"
          chip={{ text: `${d.jobCounts.enabled} enabled`, tone: "neutral" }}
        />
        <StatCard
          label="Runs · 24h"
          value={d.last24h}
          href="/jobs"
          foot="Recent executions"
          chip={{ text: `${d.runStats.successRate}% ok`, tone: d.runStats.successRate >= 80 ? "up" : "down" }}
        />
        <StatCard
          label="Attention"
          value={d.serverCounts.offline}
          href="/servers"
          foot={d.serverCounts.offline > 0 ? "Servers offline" : "All healthy"}
          chip={d.serverCounts.offline > 0 ? { text: "review", tone: "down" } : { text: "all clear", tone: "up" }}
        />

        {/* Row 2 — run activity + heads up + jobs (tall) */}
        <section className="panel chart-card span-2">
          <div className="card-head">
            <div>
              <div className="card-title">Run activity</div>
              <div className="subtle" style={{ fontSize: 12 }}>Executions over the last 7 days</div>
            </div>
            <span className="pill">{d.runStats.total} sampled</span>
          </div>
          <BarChart data={d.weekly} highlightIndex={d.todayIndex} />
        </section>

        <section className="panel">
          <div className="card-head"><div className="card-title">Heads up</div></div>
          {d.serverCounts.offline > 0 ? (
            <>
              <p style={{ fontSize: 22, fontWeight: 800, color: "var(--text)", margin: "4px 0" }}>
                {d.serverCounts.offline} offline
              </p>
              <p className="subtle" style={{ fontSize: 13 }}>
                Jobs keep running locally on those agents, but they can&apos;t sync until they reconnect.
              </p>
              <Link href="/servers" className="button block" style={{ marginTop: 14 }}>Review servers</Link>
            </>
          ) : (
            <>
              <p style={{ fontSize: 22, fontWeight: 800, color: "var(--text)", margin: "4px 0" }}>All clear</p>
              <p className="subtle" style={{ fontSize: 13 }}>
                Every agent is reporting in. Add a new scheduled job whenever you&apos;re ready.
              </p>
              <Link href={newJobHref} className="button block" style={{ marginTop: 14 }}><IconPlus /> New job</Link>
            </>
          )}
        </section>

        <section className="panel row-span-2">
          <div className="card-head">
            <div className="card-title">Jobs</div>
            <Link href="/jobs" className="button ghost sm">View all <IconChevronRight /></Link>
          </div>
          {d.jobs.length === 0 ? (
            <div className="empty">No jobs yet.</div>
          ) : (
            <div className="mini-list">
              {d.jobs.slice(0, 7).map((j) => (
                <Link href={`/jobs/${j.id}`} key={j.id} className="mini-row" style={{ color: "inherit" }}>
                  <span className="mini-icon"><IconJobs /></span>
                  <span className="mini-body">
                    <span className="mini-title">{j.name}</span>
                    <span className="mini-sub"><code>{j.schedule_cron}</code> · {targetSummary(j)}</span>
                  </span>
                  <span className={`status ${j.enabled ? "ok" : "neutral"}`}>{j.enabled ? "on" : "off"}</span>
                </Link>
              ))}
            </div>
          )}
        </section>

        {/* Row 3 — servers + gauge + deep card */}
        <section className="panel span-2">
          <div className="card-head">
            <div className="card-title">Fleet</div>
            <Link href="/servers/new" className="button ghost sm"><IconPlus /> Add server</Link>
          </div>
          {d.servers.length === 0 ? (
            <div className="empty">No servers enrolled yet.</div>
          ) : (
            <div className="mini-list">
              {d.servers.slice(0, 5).map((s) => (
                <Link href={`/servers/${s.id}`} key={s.id} className="mini-row" style={{ color: "inherit" }}>
                  <span className="mini-icon"><IconServer /></span>
                  <span className="mini-body">
                    <span className="mini-title">{s.name}</span>
                    <span className="mini-sub">{s.os || "unknown"} / {s.arch || "unknown"}</span>
                  </span>
                  <span className={`status ${serverTone[s.status]}`}>{s.status}</span>
                </Link>
              ))}
            </div>
          )}
        </section>

        <section className="panel">
          <div className="card-head"><div className="card-title">Health</div></div>
          <Gauge
            value={d.runStats.successRate}
            centerLabel={d.runStats.total > 0 ? "Success rate" : "No runs yet"}
            legend={[
              { label: "OK", value: d.runStats.succeeded, color: "var(--green)" },
              { label: "Running", value: d.runStats.running, color: "var(--warn)" },
              { label: "Failed", value: d.runStats.failed, color: "var(--danger)" },
            ]}
          />
        </section>

        <section className="deep-card span-2">
          <span className="dc-label"><span className="live-dot" />Control plane host</span>
          {host ? (
            <>
              <div className="dc-value" style={{ fontSize: 22, marginTop: 8 }}>
                {host.hostname || "localhost"}
                <span style={{ fontSize: 13, fontWeight: 600, color: "#9fc6b1", marginLeft: 8 }}>
                  {host.os}/{host.arch} · {host.cpus} CPU
                </span>
              </div>
              <div className="host-meters">
                <div className="host-meter">
                  <div className="host-meter-top">
                    <span>CPU</span>
                    <span>{host.cpu_percent.toFixed(0)}%</span>
                  </div>
                  <div className="host-meter-track"><span style={{ width: `${Math.min(100, host.cpu_percent)}%` }} /></div>
                </div>
                <div className="host-meter">
                  <div className="host-meter-top">
                    <span>Memory</span>
                    <span>{formatBytes(host.mem_used_bytes)} / {formatBytes(host.mem_total_bytes)}</span>
                  </div>
                  <div className="host-meter-track"><span style={{ width: `${Math.min(100, host.mem_percent)}%` }} /></div>
                </div>
                <div className="host-meter">
                  <div className="host-meter-top">
                    <span>Disk</span>
                    <span>{formatBytes(host.disk_used_bytes)} / {formatBytes(host.disk_total_bytes)}</span>
                  </div>
                  <div className="host-meter-track"><span style={{ width: `${Math.min(100, host.disk_percent)}%` }} /></div>
                </div>
              </div>
              <div className="dc-sub" style={{ marginTop: 10 }}>
                load {host.load1?.toFixed(2) ?? "—"} / {host.load5?.toFixed(2) ?? "—"} / {host.load15?.toFixed(2) ?? "—"}
                {" · "}uptime {formatUptime(host.uptime_sec)}
                {" · "}{d.serverCounts.online}/{d.serverCounts.total} agents online
              </div>
            </>
          ) : (
            <>
              <div className="dc-value">{d.serverCounts.online}<span style={{ fontSize: 16, fontWeight: 600, color: "#9fc6b1" }}> / {d.serverCounts.total}</span></div>
              <div className="dc-sub">agents online and syncing</div>
            </>
          )}
        </section>

        {/* Charts row */}
        <section className="panel chart-card span-2">
          <div className="card-head">
            <div>
              <div className="card-title">Volume trend</div>
              <div className="subtle" style={{ fontSize: 12 }}>Area chart of sampled runs this week</div>
            </div>
          </div>
          <AreaChart data={d.weekly} />
        </section>

        <section className="panel chart-card span-2">
          <div className="card-head">
            <div>
              <div className="card-title">Activity heatmap</div>
              <div className="subtle" style={{ fontSize: 12 }}>Runs by weekday × hour</div>
            </div>
          </div>
          <Heatmap cells={d.heatmap} />
        </section>

        <section className="panel chart-card">
          <div className="card-head">
            <div>
              <div className="card-title">Duration scatter</div>
              <div className="subtle" style={{ fontSize: 12 }}>Run length vs recency</div>
            </div>
          </div>
          {d.scatter.length === 0 ? <div className="empty">No runs to plot.</div> : <ScatterPlot points={d.scatter} />}
        </section>

        <section className="panel chart-card">
          <div className="card-head">
            <div>
              <div className="card-title">Fleet radar</div>
              <div className="subtle" style={{ fontSize: 12 }}>Online · success · jobs</div>
            </div>
          </div>
          <RadarChart axes={d.radar} />
        </section>

        <section className="panel chart-card span-2">
          <div className="card-head">
            <div>
              <div className="card-title">Server bubbles</div>
              <div className="subtle" style={{ fontSize: 12 }}>Jobs × runs · size = status weight</div>
            </div>
          </div>
          {d.bubbles.length === 0 ? <div className="empty">Add a server to plot.</div> : <BubbleChart bubbles={d.bubbles} />}
        </section>

        <section className="panel chart-card span-2">
          <div className="card-head">
            <div>
              <div className="card-title">Run share</div>
              <div className="subtle" style={{ fontSize: 12 }}>Treemap of sampled runs by server</div>
            </div>
          </div>
          {d.treemap.length === 0 ? <div className="empty">No servers yet.</div> : <Treemap nodes={d.treemap} />}
        </section>
      </div>
    </>
  );
}
