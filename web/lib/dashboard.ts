// Aggregates the data the dashboard needs from the control-plane REST API.
// There is no global runs feed, so recent-run metrics are sampled from the most
// recent jobs (bounded + parallel). Every call degrades gracefully on error.
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Run, Server, UpdateStatus } from "@/lib/types";

const SAMPLE_JOBS = 12; // cap on jobs we pull runs for

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

  let updates: UpdateStatus | null = null;
  try {
    updates = await apiGet<UpdateStatus>("/updates");
  } catch {
    updates = null;
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
  };
}
