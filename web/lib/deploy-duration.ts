/** Format a duration in milliseconds for deploy result banners. */
export function formatDeployDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const sec = Math.round(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m < 60) return s > 0 ? `${m}m ${s}s` : `${m}m`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return rm > 0 ? `${h}h ${rm}m` : `${h}h`;
}

export function deployDurationMs(startedAt?: string | null, finishedAt?: string | null): number | null {
  if (!startedAt || !finishedAt) return null;
  const ms = +new Date(finishedAt) - +new Date(startedAt);
  return Number.isFinite(ms) && ms >= 0 ? ms : null;
}

export function shortCommit(sha?: string | null): string {
  if (!sha) return "";
  return sha.length > 7 ? sha.slice(0, 7) : sha;
}
