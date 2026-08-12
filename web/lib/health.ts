// Checks the control plane's liveness/db status. Kept separate from lib/api.ts
// because /healthz is a top-level route on the control plane, not under
// /api/v1 — it predates the API and isn't covered by next.config's /api
// rewrite, so we hit the control plane origin directly.
import { apiOrigin } from "./apiBase";

export type HealthStatus = "ok" | "db_down" | "unreachable";

function healthzURL(): string {
  return `${apiOrigin()}/healthz`;
}

export async function getHealth(): Promise<HealthStatus> {
  try {
    const res = await fetch(healthzURL(), {
      cache: "no-store",
      signal: AbortSignal.timeout(2500),
    });
    if (res.ok) return "ok";
    const body = (await res.json().catch(() => null)) as { db?: string } | null;
    return body?.db === "down" ? "db_down" : "unreachable";
  } catch {
    // Control plane itself didn't respond (not started, wrong port, etc).
    return "unreachable";
  }
}
