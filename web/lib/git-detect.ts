import { apiErrorMessage } from "./api-error";
import type { DeployDetection, DeploySpecResult, GitBranch } from "./types";

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

/** Lists croncompose.yml paths found anywhere in the repo tree. */
export async function listSpecFiles(
  provider: string,
  repo: string,
  branch: string,
): Promise<string[]> {
  const q = new URLSearchParams({ provider, repo, branch });
  const res = await fetch(`/api/git/specs?${q}`);
  if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not list croncompose.yml files"));
  const data = (await res.json()) as { items?: { path: string }[] };
  return (data.items || []).map((i) => i.path);
}

/** Fetches and parses one croncompose.yml from the repo. */
export async function fetchSpecFile(
  provider: string,
  repo: string,
  branch: string,
  path: string,
): Promise<DeploySpecResult> {
  const q = new URLSearchParams({ provider, repo, branch, path });
  const res = await fetch(`/api/git/spec?${q}`);
  if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not read croncompose.yml"));
  return (await res.json()) as DeploySpecResult;
}
