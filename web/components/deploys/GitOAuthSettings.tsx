"use client";

import { useState } from "react";
import type { OAuthSettings } from "@/lib/types";

// Lets an admin plug in GitHub/GitLab OAuth app credentials from here instead of
// editing .env and restarting the control plane. A provider with no row here (or an
// empty client_id) falls back to whatever GITHUB_OAUTH_*/GITLAB_OAUTH_* env vars the
// control plane booted with, so this is additive: nothing breaks for a deployment
// that already sets those.
const PROVIDERS: Array<{ id: "github" | "gitlab"; label: string; hasBaseURL: boolean }> = [
  { id: "github", label: "GitHub", hasBaseURL: false },
  { id: "gitlab", label: "GitLab", hasBaseURL: true },
];

type Draft = {
  client_id: string;
  client_secret: string;
  redirect_url: string;
  base_url: string;
};

function draftFrom(s?: OAuthSettings): Draft {
  return {
    client_id: s?.client_id ?? "",
    client_secret: "",
    redirect_url: s?.redirect_url ?? "",
    base_url: s?.base_url ?? "",
  };
}

export function GitOAuthSettings({ initial }: { initial: OAuthSettings[] }) {
  const [settings, setSettings] = useState<Record<string, OAuthSettings | undefined>>(() => {
    const map: Record<string, OAuthSettings | undefined> = {};
    for (const s of initial) map[s.provider] = s;
    return map;
  });
  const [drafts, setDrafts] = useState<Record<string, Draft>>(() => {
    const map: Record<string, Draft> = {};
    for (const p of PROVIDERS) map[p.id] = draftFrom(settings[p.id]);
    return map;
  });
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<Record<string, string | undefined>>({});
  const [saved, setSaved] = useState<Record<string, boolean>>({});

  function setDraft(provider: string, patch: Partial<Draft>) {
    setDrafts((d) => ({ ...d, [provider]: { ...d[provider], ...patch } }));
    setSaved((s) => ({ ...s, [provider]: false }));
  }

  async function save(provider: string) {
    setBusy(provider);
    setError((e) => ({ ...e, [provider]: undefined }));
    try {
      const draft = drafts[provider];
      const res = await fetch(`/api/auth/oauth-settings/${provider}`, {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          client_id: draft.client_id.trim(),
          client_secret: draft.client_secret,
          redirect_url: draft.redirect_url.trim(),
          base_url: draft.base_url.trim(),
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      const next = (await res.json()) as OAuthSettings;
      setSettings((s) => ({ ...s, [provider]: next }));
      setDrafts((d) => ({ ...d, [provider]: draftFrom(next) }));
      setSaved((s) => ({ ...s, [provider]: true }));
    } catch (e) {
      setError((err) => ({ ...err, [provider]: (e as Error).message }));
    } finally {
      setBusy(null);
    }
  }

  async function clear(provider: string) {
    if (!confirm(`Revert ${provider === "github" ? "GitHub" : "GitLab"} to the .env configuration?`)) return;
    setBusy(provider);
    setError((e) => ({ ...e, [provider]: undefined }));
    try {
      const res = await fetch(`/api/auth/oauth-settings/${provider}`, { method: "DELETE" });
      if (!res.ok && res.status !== 204) throw new Error(await res.text());
      setSettings((s) => ({ ...s, [provider]: undefined }));
      setDrafts((d) => ({ ...d, [provider]: draftFrom(undefined) }));
      setSaved((s) => ({ ...s, [provider]: false }));
    } catch (e) {
      setError((err) => ({ ...err, [provider]: (e as Error).message }));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="stack">
      {PROVIDERS.map((p) => {
        const s = settings[p.id];
        const draft = drafts[p.id];
        const status = s?.configured ? "configured here" : "using .env (or not set)";
        return (
          <div className="panel" key={p.id}>
            <div className="row" style={{ alignItems: "flex-start" }}>
              <div>
                <div style={{ fontWeight: 700 }}>{p.label} OAuth app</div>
                <div className="subtle" style={{ fontSize: 12 }}>{status}</div>
              </div>
              {s?.configured && (
                <button
                  type="button"
                  className="button secondary sm"
                  disabled={busy === p.id}
                  onClick={() => clear(p.id)}
                >
                  Revert to .env
                </button>
              )}
            </div>

            <div className="grid-2" style={{ marginTop: 12 }}>
              <div className="field">
                <label htmlFor={`${p.id}-client-id`}>Client ID</label>
                <input
                  id={`${p.id}-client-id`}
                  value={draft.client_id}
                  onChange={(e) => setDraft(p.id, { client_id: e.target.value })}
                  autoComplete="off"
                />
              </div>
              <div className="field">
                <label htmlFor={`${p.id}-client-secret`}>Client secret</label>
                <input
                  id={`${p.id}-client-secret`}
                  type="password"
                  value={draft.client_secret}
                  onChange={(e) => setDraft(p.id, { client_secret: e.target.value })}
                  placeholder={s?.has_secret ? "Leave blank to keep the current secret" : ""}
                  autoComplete="new-password"
                />
              </div>
            </div>

            <div className="field">
              <label htmlFor={`${p.id}-redirect`}>Callback URL</label>
              <input
                id={`${p.id}-redirect`}
                value={draft.redirect_url}
                onChange={(e) => setDraft(p.id, { redirect_url: e.target.value })}
                placeholder="Leave blank to use this control plane's public URL"
                autoComplete="off"
              />
              <p className="field-hint">
                Register this exact URL as the app&apos;s callback in {p.label}&apos;s OAuth app settings.
              </p>
            </div>

            {p.hasBaseURL && (
              <div className="field">
                <label htmlFor={`${p.id}-base`}>GitLab base URL</label>
                <input
                  id={`${p.id}-base`}
                  value={draft.base_url}
                  onChange={(e) => setDraft(p.id, { base_url: e.target.value })}
                  placeholder="https://gitlab.com (or your self-hosted instance)"
                  autoComplete="off"
                />
              </div>
            )}

            {error[p.id] && <p className="form-error">{error[p.id]}</p>}
            <button
              type="button"
              className="button sm"
              disabled={busy === p.id || !draft.client_id.trim()}
              onClick={() => save(p.id)}
              style={{ marginTop: 8 }}
            >
              {busy === p.id ? "Saving…" : saved[p.id] ? "Saved" : "Save"}
            </button>
          </div>
        );
      })}
    </div>
  );
}
