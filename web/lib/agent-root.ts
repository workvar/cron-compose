export class ControlPlaneError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ControlPlaneError";
  }
}

export function parseControlPlaneError(status: number, text: string, fallback: string): ControlPlaneError {
  let message = text || `${fallback} (HTTP ${status})`;
  let code = "";
  try {
    const parsed = JSON.parse(text) as { error?: { message?: string; code?: string } };
    if (parsed?.error?.message) message = parsed.error.message;
    if (parsed?.error?.code) code = parsed.error.code;
  } catch { /* keep fallback */ }
  return new ControlPlaneError(status, code, message);
}

export function isAgentOfflineError(err: unknown): boolean {
  return err instanceof ControlPlaneError && err.code === "agent_offline";
}

export type AgentRootView = {
  label: "Off" | "Enabling" | "On (root)" | "On (waiting)" | "Error";
  tone: "neutral" | "ok" | "warn" | "danger";
};

export function agentRootView(input: {
  enabled: boolean;
  euidRoot: boolean;
  busy: boolean;
  error?: string | null;
}): AgentRootView {
  if (input.error) return { label: "Error", tone: "danger" };
  if (input.busy) return { label: "Enabling", tone: "warn" };
  if (!input.enabled) return { label: "Off", tone: "neutral" };
  if (input.euidRoot) return { label: "On (root)", tone: "ok" };
  return { label: "On (waiting)", tone: "warn" };
}

export function visibleStoredRootError(input: {
  enabled: boolean;
  euidRoot: boolean;
  storedError?: string | null;
}): string | null {
  const stored = input.storedError?.trim() ? input.storedError : null;
  if (!stored) return null;
  if (input.enabled === input.euidRoot) return null;
  return stored;
}

/** True while the DB flag and reported euid disagree — agent is restarting or stuck. */
export function agentRootPending(enabled: boolean, euidRoot: boolean): boolean {
  return enabled !== euidRoot;
}

export const AGENT_ROOT_WATCH_MS = 60_000;
export const AGENT_ROOT_POLL_MS = 2_000;

export function agentRootTimeoutMessage(enabled: boolean, storedError?: string | null): string {
  const stored = storedError?.trim();
  if (stored) return stored;
  if (enabled) {
    return "Agent did not report root after 60s. On the host check: systemctl status croncompose-agent; " +
      "ls /etc/systemd/system/croncompose-agent.service.d/; " +
      "and that sudo -n /usr/libexec/croncompose/agent-privctl elevate works for the agent user.";
  }
  return "Agent is still running as root after demote. Check systemctl status croncompose-agent and remove " +
    "/etc/systemd/system/croncompose-agent.service.d/root.conf if it remains.";
}

export type AgentRootWatchDecision =
  | { action: "continue" }
  | { action: "done"; enabled: boolean; euidRoot: boolean }
  | { action: "timeout"; error: string };

/** Pure step for the waiting poll loop (used by AgentRootToggle). */
export function decideAgentRootWatch(input: {
  enabled: boolean;
  euidRoot: boolean;
  elapsedMs: number;
  timeoutMs?: number;
  storedError?: string | null;
}): AgentRootWatchDecision {
  if (!agentRootPending(input.enabled, input.euidRoot)) {
    return { action: "done", enabled: input.enabled, euidRoot: input.euidRoot };
  }
  const limit = input.timeoutMs ?? AGENT_ROOT_WATCH_MS;
  if (input.elapsedMs >= limit) {
    return { action: "timeout", error: agentRootTimeoutMessage(input.enabled, input.storedError) };
  }
  // Surface a privctl failure as soon as the control plane has one — don't wait out the full timeout.
  const stored = input.storedError?.trim();
  if (stored) {
    return { action: "timeout", error: stored };
  }
  return { action: "continue" };
}

export function applyToggleFailure(input: {
  requestedEnabled: boolean;
  previousEnabled: boolean;
  previousEuidRoot: boolean;
  error: unknown;
}): { enabled: boolean; euidRoot: boolean; error: string | null } {
  if (isAgentOfflineError(input.error)) {
    return {
      enabled: input.requestedEnabled,
      euidRoot: input.requestedEnabled ? false : input.previousEuidRoot,
      error: null,
    };
  }
  const message = input.error instanceof Error ? input.error.message : "Could not change agent root access";
  return {
    enabled: input.previousEnabled,
    euidRoot: input.previousEuidRoot,
    error: message,
  };
}
