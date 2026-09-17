// Per-server "agent update in progress" tracker, kept in sessionStorage so the
// state survives a page refresh. This is separate from lib/updating.ts, which
// drives the full-screen overlay for whole-stack rebuilds: a single-agent
// update should not black out the whole dashboard, but it still needs to
// remember it's running across a reload so the button doesn't flip back to
// "Update" while the agent is mid-rebuild.

const PREFIX = "cc-agent-updating:";

export type AgentUpdatingState = {
  targetVersion: string;
  startedAt: number;
  phase?: string;
  detail?: string;
  percent?: number;
};

export function beginAgentUpdating(serverId: string, targetVersion: string): void {
  writeAgentUpdating(serverId, { targetVersion, startedAt: Date.now(), phase: "offered", detail: "Sending the update command to the agent", percent: 5 });
}

export function patchAgentUpdating(serverId: string, patch: Partial<Pick<AgentUpdatingState, "phase" | "detail" | "percent">>): void {
  const current = readAgentUpdating(serverId);
  if (!current) return;
  writeAgentUpdating(serverId, { ...current, ...patch });
}

function writeAgentUpdating(serverId: string, state: AgentUpdatingState): void {
  if (typeof window === "undefined") return;
  try {
    sessionStorage.setItem(PREFIX + serverId, JSON.stringify(state));
  } catch {
    // sessionStorage unavailable (e.g. private mode) — polling still works
    // for the current mount, it just won't survive a reload.
  }
}

export function readAgentUpdating(serverId: string): AgentUpdatingState | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = sessionStorage.getItem(PREFIX + serverId);
    if (!raw) return null;
    return JSON.parse(raw) as AgentUpdatingState;
  } catch {
    return null;
  }
}

export function clearAgentUpdating(serverId: string): void {
  if (typeof window === "undefined") return;
  try {
    sessionStorage.removeItem(PREFIX + serverId);
  } catch {
    // ignore
  }
}
