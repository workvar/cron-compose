/** Map pm2/systemd/docker process states onto status pill tones. */
export function processStateTone(state?: string | null): string {
  const s = (state || "").toLowerCase().trim();
  switch (s) {
    case "online":
    case "running":
    case "active":
    case "listening":
      return "ok";
    case "launching":
    case "stopping":
    case "waiting restart":
    case "one-launch-status":
      return "info";
    case "stopped":
    case "offline":
    case "inactive":
    case "exited":
      return "neutral";
    case "errored":
    case "error":
    case "failed":
    case "unreachable":
      return "danger";
    default:
      return s ? "neutral" : "neutral";
  }
}

export function processStateLabel(state?: string | null): string {
  const s = (state || "").trim();
  if (!s) return "unknown";
  return s;
}
