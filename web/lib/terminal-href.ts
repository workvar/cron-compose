/** Build the chrome-less fullscreen terminal URL (includes /app basePath for raw opens). */
export function terminalFullscreenPath(
  serverId: string,
  opts: { mode: "shell" | "command"; command?: string; runAs?: string },
): string {
  const q = new URLSearchParams();
  q.set("mode", opts.mode);
  if (opts.mode === "command" && opts.command?.trim()) q.set("command", opts.command.trim());
  if (opts.runAs?.trim()) q.set("runAs", opts.runAs.trim());
  const qs = q.toString();
  return `/app/servers/${serverId}/terminal/full${qs ? `?${qs}` : ""}`;
}

/** Open the fullscreen terminal in a new browser tab. */
export function openTerminalFullscreen(
  serverId: string,
  opts: { mode: "shell" | "command"; command?: string; runAs?: string },
): void {
  window.open(terminalFullscreenPath(serverId, opts), "_blank", "noopener,noreferrer");
}
