export type GitOAuthProvider = "github" | "gitlab";

export type GitOAuthConnectAction = "pending" | "setup" | "ask-admin" | "start";

export function gitOAuthStartUrl(origin: string, provider: GitOAuthProvider, next: string): string {
  const u = new URL(`/api/v1/auth/${provider}/start`, origin);
  u.searchParams.set("purpose", "git");
  u.searchParams.set("next", next);
  return u.toString();
}

export function gitOAuthConnectAction(opts: {
  enabled?: boolean;
  configLoaded: boolean;
  isAdmin: boolean;
}): GitOAuthConnectAction {
  if (!opts.configLoaded) return "pending";
  if (opts.enabled === false) return opts.isAdmin ? "setup" : "ask-admin";
  return "start";
}

export function oauthAppSaveReady(clientId: string, clientSecret: string, hasStoredSecret: boolean): boolean {
  if (!clientId.trim()) return false;
  return hasStoredSecret || clientSecret.trim() !== "";
}

export function defaultOAuthCallbackUrl(origin: string, provider: GitOAuthProvider): string {
  return `${origin.replace(/\/$/, "")}/api/auth/${provider}/callback`;
}

export type GitOAuthStartFetchAction = "navigate" | "setup" | "ask-admin" | "error";

// fetch(startUrl, { redirect: "manual" }) turns a 302 to GitHub into an opaque
// redirect (status 0). That is success — navigate — not "OAuth is not configured".
export function gitOAuthStartFetchAction(opts: {
  ok: boolean;
  status: number;
  type?: string;
  errorMessage?: string;
  isAdmin: boolean;
}): GitOAuthStartFetchAction {
  if (opts.type === "opaqueredirect" || opts.status === 0 || (opts.status >= 300 && opts.status < 400) || opts.ok) {
    return "navigate";
  }
  const msg = opts.errorMessage || "";
  if (/not configured|oauth/i.test(msg)) {
    return opts.isAdmin ? "setup" : "ask-admin";
  }
  return "error";
}
