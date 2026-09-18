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
