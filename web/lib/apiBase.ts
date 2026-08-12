// Where the control plane's REST API lives, resolved per call rather than at module
// load or build time. The standalone bundle is built once and then started by pm2
// with the ports the installer picked, so baking the value in would pin the UI to
// whatever port happened to be set during `next build` (the cause of a UI that
// renders but reports "could not reach the control plane").
//
// Server-only: nothing in the browser bundle should import this.
export function apiBase(): string {
  const explicit = process.env.API_BASE?.trim();
  if (explicit) return explicit.replace(/\/+$/, "");
  const port = process.env.CC_API_PORT?.trim() || "8080";
  return `http://127.0.0.1:${port}/api/v1`;
}

// Origin of the control plane, for routes that sit outside /api/v1 (e.g. /healthz).
export function apiOrigin(): string {
  return new URL(apiBase()).origin;
}
