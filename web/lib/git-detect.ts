import { apiErrorMessage } from "./api-error";
import type { DeployDetection, GitBranch } from "./types";

/**
 * Re-runs detection scoped to one subfolder of the repo (see DetectAt on the
 * control plane). Used when a project block's root folder changes, so a Go API
 * under backend/ gets its own language/install guess instead of inheriting
 * whatever the repo root looked like.
 */
export async function detectAt(
  provider: string,
  repo: string,
  branch: string,
  root: string,
): Promise<DeployDetection> {
  const q = new URLSearchParams({ provider, repo, branch, path: root === "." ? "" : root });
  const res = await fetch(`/api/git/inspect?${q}`);
  if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not detect this folder"));
  return (await res.json()) as DeployDetection;
}

/** Lists a repo's branches for the import wizard's searchable branch picker. */
export async function listBranches(provider: string, repo: string): Promise<GitBranch[]> {
  const q = new URLSearchParams({ provider, repo });
  const res = await fetch(`/api/git/branches?${q}`);
  if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not list branches"));
  const data = (await res.json()) as { items?: GitBranch[] };
  return data.items || [];
}
