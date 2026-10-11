/** Deploy analytics API shapes (control-plane /deploys/.../analytics). */

export type DeployTrendPoint = { label: string; value: number; at?: string };
export type DeployPhasePoint = { phase: string; ms: number };
export type DeployProjectRollup = {
  project_id: string;
  name: string;
  runs: number;
  success_rate: number;
  avg_duration_ms: number;
  avg_release_bytes: number;
};
export type DeployAnalyticsRun = {
  id: string;
  project_id: string;
  project_name?: string;
  status: string;
  branch: string;
  commit_sha?: string;
  duration_ms: number;
  release_bytes?: number;
  phase_ms?: Record<string, number>;
  created_at: string;
};

export type DeployAnalytics = {
  runs_total: number;
  succeeded: number;
  failed: number;
  success_rate: number;
  avg_duration_ms: number;
  avg_release_bytes: number;
  duration_trend: DeployTrendPoint[];
  release_trend: DeployTrendPoint[];
  phase_avg_ms: DeployPhasePoint[];
  by_project?: DeployProjectRollup[];
  recent: DeployAnalyticsRun[];
};

export function formatDurationMs(ms: number): string {
  if (!ms || ms <= 0) return "—";
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(s >= 10 ? 0 : 1)}s`;
  const m = Math.floor(s / 60);
  const rem = Math.round(s % 60);
  return rem ? `${m}m ${rem}s` : `${m}m`;
}

export function formatBytes(n: number): string {
  if (!n || n <= 0) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function phaseLabel(phase: string): string {
  switch (phase) {
    case "preflight":
      return "Preflight";
    case "clone":
      return "Clone";
    case "install":
      return "Build";
    case "pre_start":
      return "Pre-start";
    case "release":
      return "Activate";
    case "start":
      return "Start";
    case "health":
      return "Health";
    default:
      return phase;
  }
}
