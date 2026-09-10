"use client";

import { useEffect, useState } from "react";
import type { GitConnection } from "@/lib/types";

const PROVIDERS: Array<{ id: "github" | "gitlab"; label: string }> = [
  { id: "github", label: "GitHub" },
  { id: "gitlab", label: "GitLab" },
];

type AuthConfig = {
  github_enabled?: boolean;
  gitlab_enabled?: boolean;
};

export function GitConnections({
  initial,
  next = "/app/settings",
}: {
  initial: GitConnection[];
  next?: string;
}) {
  const [items, setItems] = useState(initial);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [cfg, setCfg] = useState<AuthConfig>({});

  useEffect(() => {
    fetch("/api/auth/config")
      .then((r) => r.json() as Promise<AuthConfig>)
      .then(setCfg)
      .catch(() => {});
  }, []);

  async function connect(provider: "github" | "gitlab") {
    setError(null);
    const enabled = provider === "github" ? cfg.github_enabled : cfg.gitlab_enabled;
    if (enabled === false) {
      setError(`${provider === "github" ? "GitHub" : "GitLab"} OAuth is not configured on this control plane.`);
      return;
    }
    const u = new URL(`/api/v1/auth/${provider}/start`, window.location.origin);
    u.searchParams.set("purpose", "git");
    u.searchParams.set("next", next);
    const res = await fetch(u.toString(), { redirect: "manual", credentials: "include" });
    if (res.status >= 300 && res.status < 400) {
      const loc = res.headers.get("location");
      window.location.href = loc || u.toString();
      return;
    }
    if (!res.ok) {
      let msg = `${provider} OAuth is not configured`;
      try {
        const j = (await res.json()) as { error?: { message?: string } };
        if (j.error?.message) msg = j.error.message;
      } catch { /* ignore */ }
      setError(msg);
      return;
    }
    window.location.href = u.toString();
  }

  async function disconnect(provider: string) {
    setBusy(provider);
    setError(null);
    try {
      const res = await fetch(`/api/git/connections/${provider}`, { method: "DELETE" });
      if (!res.ok) throw new Error("disconnect failed");
      setItems((prev) => prev.filter((c) => c.provider !== provider));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="stack">
      {PROVIDERS.map((p) => {
        const linked = items.find((c) => c.provider === p.id);
        return (
          <div className="panel" key={p.id}>
            <div className="row">
              <div>
                <div style={{ fontWeight: 700 }}>{p.label}</div>
                <div className="subtle" style={{ fontSize: 13 }}>
                  {linked ? `Connected as ${linked.login}` : "Not connected"}
                </div>
              </div>
              {linked ? (
                <button
                  type="button"
                  className="button secondary sm"
                  disabled={busy === p.id}
                  onClick={() => disconnect(p.id)}
                >
                  Disconnect
                </button>
              ) : (
                <button type="button" className="button sm" onClick={() => connect(p.id)}>
                  Connect {p.label}
                </button>
              )}
            </div>
          </div>
        );
      })}
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}
