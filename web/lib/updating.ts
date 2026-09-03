const KEY = "cc-updating";

export type UpdatingState = {
  targetVersion: string;
  startedAt: number;
  /** When true, wait for the control-plane stack to come back. */
  stack: boolean;
  serverIds: string[];
};

export function beginUpdating(
  targetVersion: string,
  opts: { stack?: boolean; serverIds?: string[] } = {},
): void {
  if (typeof window === "undefined") return;
  const state: UpdatingState = {
    targetVersion,
    startedAt: Date.now(),
    stack: opts.stack ?? false,
    serverIds: opts.serverIds ?? [],
  };
  sessionStorage.setItem(KEY, JSON.stringify(state));
  window.dispatchEvent(new CustomEvent("cc-updating", { detail: state }));
}

export function readUpdating(): UpdatingState | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return null;
    return JSON.parse(raw) as UpdatingState;
  } catch {
    return null;
  }
}

export function clearUpdating(): void {
  if (typeof window === "undefined") return;
  sessionStorage.removeItem(KEY);
  window.dispatchEvent(new CustomEvent("cc-updating", { detail: null }));
}
