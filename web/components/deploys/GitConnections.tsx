"use client";

import { useEffect, useState } from "react";
import type { GitConnection } from "@/lib/types";
import { GitOAuthSetupModal } from "@/components/deploys/GitOAuthSetupModal";
import { gitOAuthConnectAction, gitOAuthStartUrl } from "@/lib/git-oauth";

const PROVIDERS: Array<{ id: "github" | "gitlab"; label: string }> = [
  { id: "github", label: "GitHub" },
  { id: "gitlab", label: "GitLab" },
];

type AuthConfig = {
  github_enabled?: boolean;
  gitlab_enabled?: boolean;
};

type MeLite = { role?: string };

function isAdminRole(role?: string) {
  return role === "admin" || role === "owner";
}

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
  const [cfgLoaded, setCfgLoaded] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false);
  const [setupProvider, setSetupProvider] = useState<"github" | "gitlab" | null>(null);

  useEffect(() => {
    fetch("/api/auth/config", { credentials: "include" })
      .then((r) => r.json() as Promise<AuthConfig>)
      .then((c) => {
        setCfg(c);
        setCfgLoaded(true);
      })
      .catch(() => setCfgLoaded(true));
    fetch("/api/me", { credentials: "include" })
      .then((r) => (r.ok ? (r.json() as Promise<MeLite>) : null))
      .then((me) => setIsAdmin(isAdminRole(me?.role)))
      .catch(() => setIsAdmin(false));
  }, []);

  function markEnabled(provider: "github" | "gitlab") {
    setCfg((c) =>
      provider === "github" ? { ...c, github_enabled: true } : { ...c, gitlab_enabled: true },
    );
    setCfgLoaded(true);
  }

  function beginOAuth(provider: "github" | "gitlab") {
    // Full-page navigation, same as Sign in with GitHub. fetch() + redirect:"manual"
    // turns the 302 to the provider into an opaque status-0 response, which the old
    // Connect handler treated as "not configured" and reopened the setup modal.
    window.location.href = gitOAuthStartUrl(window.location.origin, provider, next);
  }

  function connect(provider: "github" | "gitlab") {
    setError(null);
    const enabled = provider === "github" ? cfg.github_enabled : cfg.gitlab_enabled;
    const action = gitOAuthConnectAction({ enabled, configLoaded: cfgLoaded, isAdmin });
    if (action === "pending") return;
    if (action === "setup") {
      setSetupProvider(provider);
      return;
    }
    if (action === "ask-admin") {
      setError(
        `${provider === "github" ? "GitHub" : "GitLab"} OAuth is not configured. Ask an admin to set it up.`,
      );
      return;
    }
    beginOAuth(provider);
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
        const enabled = p.id === "github" ? cfg.github_enabled : cfg.gitlab_enabled;
        const status = linked
          ? `Connected as ${linked.login}`
          : enabled === false
            ? "OAuth app not configured"
            : "Not connected";
        return (
          <div className="panel" key={p.id}>
            <div className="row">
              <div>
                <div style={{ fontWeight: 700 }}>{p.label}</div>
                <div className="subtle" style={{ fontSize: 13 }}>
                  {status}
                </div>
              </div>
              <div className="cluster">
                {isAdmin && !linked && enabled !== false && (
                  <button
                    type="button"
                    className="button ghost sm"
                    onClick={() => setSetupProvider(p.id)}
                  >
                    OAuth app
                  </button>
                )}
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
                  <button
                    type="button"
                    className="button sm"
                    disabled={!cfgLoaded}
                    onClick={() => void connect(p.id)}
                  >
                    Connect {p.label}
                  </button>
                )}
              </div>
            </div>
          </div>
        );
      })}
      {error && <p className="form-error">{error}</p>}
      {setupProvider && (
        <GitOAuthSetupModal
          provider={setupProvider}
          open
          onClose={() => setSetupProvider(null)}
          onSaved={(p) => {
            markEnabled(p);
            setSetupProvider(null);
            beginOAuth(p);
          }}
        />
      )}
    </div>
  );
}
