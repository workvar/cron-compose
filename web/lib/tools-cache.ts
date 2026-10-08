import type { ToolStatus } from "@/lib/types";

export type ToolsCacheEntry = {
  tools: Record<string, ToolStatus>;
  updatedAt: number;
};

const PREFIX = "cc-tools-v1:";

function key(serverId: string, runAs: string): string {
  return `${PREFIX}${serverId}:${runAs || "_agent"}`;
}

export function readToolsCache(serverId: string, runAs: string): ToolsCacheEntry | null {
  if (typeof window === "undefined" || !serverId) return null;
  try {
    const raw = sessionStorage.getItem(key(serverId, runAs));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as ToolsCacheEntry;
    if (!parsed?.tools || typeof parsed.tools !== "object") return null;
    return parsed;
  } catch {
    return null;
  }
}

export function writeToolsCache(serverId: string, runAs: string, tools: Record<string, ToolStatus>) {
  if (typeof window === "undefined" || !serverId) return;
  try {
    const entry: ToolsCacheEntry = { tools, updatedAt: Date.now() };
    sessionStorage.setItem(key(serverId, runAs), JSON.stringify(entry));
  } catch {
    /* quota / private mode */
  }
}

export function clearToolsCache(serverId: string, runAs: string) {
  if (typeof window === "undefined" || !serverId) return;
  try {
    sessionStorage.removeItem(key(serverId, runAs));
  } catch {
    /* ignore */
  }
}

export function mergeToolStatuses(
  prev: Record<string, ToolStatus>,
  incoming: ToolStatus[],
): Record<string, ToolStatus> {
  const next = { ...prev };
  for (const t of incoming) {
    next[t.name] = t;
  }
  return next;
}
