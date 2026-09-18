// Aggregates the data the dashboard needs from the control-plane REST API.
// There is no global runs feed, so recent-run metrics are sampled from the most
// recent jobs (bounded + parallel). Every call degrades gracefully on error.
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Run, Server, UpdateStatus } from "@/lib/types";

const SAMPLE_JOBS = 12; // cap on jobs we pull runs for

export type HostMetrics = {
  hostname?: string;
  os?: string;
  arch?: string;
  cpus?: number;
  uptime_sec?: number;
  load1?: number;
  load5?: number;
  load15?: number;
  cpu_percent: number;
  mem_total_bytes: number;
  mem_used_bytes: number;
  mem_percent: number;
  disk_total_bytes: number;
  disk_used_bytes: number;
  disk_percent: number;
  collected_at?: string;
};

export type DashboardData = {
  servers: Server[];
  jobs: Job[];
  serverCounts: { total: number; online: number; offline: number; pending: number };
  jobCounts: { total: number; enabled: number; disabled: number };
  recentRuns: Run[]; // sorted newest first
  runStats: { total: number; succeeded: number; running: number; failed: number; successRate: number };
  last24h: number;
  weekly: { label: string; value: number }[];
  todayIndex: number;
  reachable: boolean;
  updates: UpdateStatus | null;
  host: HostMetrics | null;
  heatmap: { day: number; hour: number; value: number }[];
  scatter: { x: number; y: number; label?: string; tone?: "ok" | "danger" | "neutral" }[];
  bubbles: { x: number; y: number; r: number; label: string; tone?: string }[];
  radar: { label: string; value: number }[];
  treemap: { label: string; value: number; color?: string }[];
};

const WEEKDAY = ["S", "M", "T", "W", "T", "F", "S"];

async function safeList<T>(path: string): Promise<T[]> {
  try {
    const data = await apiGet<ListResponse<T>>(path);
    return data.items ?? [];
  } catch {
    return [];
  }
}

function durationMs(r: Run): number {
  if (typeof r.duration_ms === "number" && r.duration_ms >= 0) return r.duration_ms;
  if (r.started_at && r.finished_at) {
    return Math.max(0, +new Date(r.finished_at) - +new Date(r.started_at));
  }
  return 0;
}

export async function getDashboardData(): Promise<DashboardData> {
  let reachable = true;
  let servers: Server[] = [];
  let jobs: Job[] = [];
  try {
    [servers, jobs] = await Promise.all([
      apiGet<ListResponse<Server>>("/servers").then((d) => d.items ?? []),
      apiGet<ListResponse<Job>>("/jobs").then((d) => d.items ?? []),
    ]);
  } catch {
    reachable = false;
  }

  // Sample recent runs across the newest jobs.
  const sample = jobs.slice(0, SAMPLE_JOBS);
  const runLists = await Promise.all(sample.map((j) => safeList<Run>(`/jobs/${j.id}/runs?limit=20`)));
  const recentRuns = runLists
    .flat()
    .sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at));

  const serverCounts = {
    total: servers.length,
    online: servers.filter((s) => s.status === "online").length,
    offline: servers.filter((s) => s.status === "offline").length,
    pending: servers.filter((s) => s.status === "pending").length,
  };
  const jobCounts = {
    total: jobs.length,
    enabled: jobs.filter((j) => j.enabled).length,
    disabled: jobs.filter((j) => !j.enabled).length,
  };

  const terminal = recentRuns.filter((r) =>
    ["succeeded", "failed", "timed_out", "canceled", "skipped"].includes(r.status),
  );
  const succeeded = recentRuns.filter((r) => r.status === "succeeded").length;
  const running = recentRuns.filter((r) => r.status === "running" || r.status === "pending").length;
  const failed = recentRuns.filter((r) => r.status === "failed" || r.status === "timed_out").length;
  const successRate = terminal.length > 0 ? Math.round((succeeded / terminal.length) * 100) : 0;

  // Last 7 calendar days of run counts.
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const dayMs = 86_400_000;
  const weekly = Array.from({ length: 7 }, (_, i) => {
    const day = new Date(startOfToday.getTime() - (6 - i) * dayMs);
    const next = new Date(day.getTime() + dayMs);
    const value = recentRuns.filter((r) => {
      const t = new Date(r.created_at);
      return t >= day && t < next;
    }).length;
    return { label: WEEKDAY[day.getDay()], value };
  });
  const last24h = recentRuns.filter((r) => +now - +new Date(r.created_at) <= dayMs).length;

  const heatmapMap = new Map<string, number>();
  for (const r of recentRuns) {
    const t = new Date(r.created_at);
    const key = `${t.getDay()}-${t.getHours()}`;
    heatmapMap.set(key, (heatmapMap.get(key) ?? 0) + 1);
  }
  const heatmap = Array.from(heatmapMap.entries()).map(([k, value]) => {
    const [day, hour] = k.split("-").map(Number);
    return { day, hour, value };
  });

  const oldest = recentRuns.length
    ? Math.min(...recentRuns.map((r) => +new Date(r.created_at)))
    : +now;
  const span = Math.max(1, +now - oldest);
  const scatter = recentRuns.slice(0, 40).map((r) => {
    const dur = durationMs(r);
    const tone =
      r.status === "succeeded" ? "ok" : r.status === "failed" || r.status === "timed_out" ? "danger" : "neutral";
    return {
      x: (+new Date(r.created_at) - oldest) / span,
      y: Math.max(dur, 1),
      label: `${r.status} · ${Math.round(dur)}ms`,
      tone: tone as "ok" | "danger" | "neutral",
    };
  });

  const runsByServer = new Map<string, number>();
  for (const r of recentRuns) {
    runsByServer.set(r.server_id, (runsByServer.get(r.server_id) ?? 0) + 1);
  }
  const jobsByServer = new Map<string, number>();
  for (const j of jobs) {
    if (j.server_id) jobsByServer.set(j.server_id, (jobsByServer.get(j.server_id) ?? 0) + 1);
  }
  const bubbles = servers.slice(0, 8).map((s) => ({
    x: Math.max(jobsByServer.get(s.id) ?? 0, 0.5),
    y: Math.max(runsByServer.get(s.id) ?? 0, 0.5),
    r: s.status === "online" ? 3 : s.status === "pending" ? 2 : 1,
    label: s.name,
    tone: s.status === "online" ? "var(--green-mint)" : s.status === "offline" ? "#f5b4b0" : "var(--surface-3)",
  }));

  const onlinePct = serverCounts.total ? (serverCounts.online / serverCounts.total) * 100 : 0;
  const enabledPct = jobCounts.total ? (jobCounts.enabled / jobCounts.total) * 100 : 0;
  const radar = [
    { label: "Online", value: onlinePct },
    { label: "Success", value: successRate },
    { label: "Enabled", value: enabledPct },
    { label: "Activity", value: Math.min(100, last24h * 10) },
    { label: "Coverage", value: Math.min(100, jobs.length * 8) },
  ];

  const treemap = servers.slice(0, 10).map((s, i) => ({
    label: s.name,
    value: Math.max(runsByServer.get(s.id) ?? 0, s.status === "online" ? 1 : 0.5),
    color: i % 2 === 0 ? "var(--green-soft)" : "#dcefe4",
  }));

  let updates: UpdateStatus | null = null;
  try {
    updates = await apiGet<UpdateStatus>("/updates");
  } catch {
    updates = null;
  }

  let host: HostMetrics | null = null;
  try {
    host = await apiGet<HostMetrics>("/system/host");
  } catch {
    host = null;
  }

  return {
    servers,
    jobs,
    serverCounts,
    jobCounts,
    recentRuns,
    runStats: { total: recentRuns.length, succeeded, running, failed, successRate },
    last24h,
    weekly,
    todayIndex: 6,
    reachable,
    updates,
    host,
    heatmap,
    scatter,
    bubbles,
    radar,
    treemap,
  };
}
