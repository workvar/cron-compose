"use client";

import { useEffect, useState } from "react";
import type { OAuthSettings } from "@/lib/types";
import { defaultOAuthCallbackUrl, oauthAppSaveReady } from "@/lib/git-oauth";

type Draft = {
  client_id: string;
  client_secret: string;
  redirect_url: string;
  base_url: string;
};

function emptyDraft(): Draft {
  return { client_id: "", client_secret: "", redirect_url: "", base_url: "" };
}

function draftFrom(s?: OAuthSettings): Draft {
  return {
    client_id: s?.client_id ?? "",
    client_secret: "",
    redirect_url: s?.redirect_url ?? "",
    base_url: s?.base_url ?? "",
  };
}

export function GitOAuthSetupModal({
  provider,
  open,
  onClose,
  onSaved,
}: {
  provider: "github" | "gitlab";
  open: boolean;
  onClose: () => void;
  onSaved: (provider: "github" | "gitlab") => void;
}) {
  const label = provider === "github" ? "GitHub" : "GitLab";
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [hasSecret, setHasSecret] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError(null);
    setLoading(true);
    fetch("/api/auth/oauth-settings", { credentials: "include" })
      .then(async (r) => {
        if (!r.ok) throw new Error(r.status === 403 ? "Only admins can configure OAuth apps." : await r.text());
        const data = (await r.json()) as { items?: OAuthSettings[] };
        const row = (data.items ?? []).find((s) => s.provider === provider);
        const next = draftFrom(row);
        if (!next.redirect_url) {
          next.redirect_url = defaultOAuthCallbackUrl(window.location.origin, provider);
        }
        setDraft(next);
        setHasSecret(!!row?.has_secret);
      })
      .catch((e) => setError((e as Error).message))
      .finally(() => setLoading(false));
  }, [open, provider]);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  async function save() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/auth/oauth-settings/${provider}`, {
        method: "PUT",
        headers: { "content-type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          client_id: draft.client_id.trim(),
          client_secret: draft.client_secret,
          redirect_url: draft.redirect_url.trim(),
          base_url: draft.base_url.trim(),
        }),
      });
      if (!res.ok) {
        let msg = await res.text();
        try {
          const j = JSON.parse(msg) as { error?: { message?: string } };
          if (j.error?.message) msg = j.error.message;
        } catch { /* keep raw body */ }
        throw new Error(msg || `Save failed (HTTP ${res.status})`);
      }
      const saved = (await res.json()) as OAuthSettings;
      if (!saved.configured) {
        throw new Error("Client ID and secret are both required.");
      }
      onSaved(provider);
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation" onClick={onClose}>
      <div
        className="modal-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="oauth-setup-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="row" style={{ alignItems: "flex-start", marginBottom: 8 }}>
          <div>
            <h2 id="oauth-setup-title" style={{ margin: 0 }}>{label} OAuth app</h2>
            <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
              Paste the OAuth app credentials. Saving sends you to {label} to connect your account.
            </p>
          </div>
          <button type="button" className="button ghost sm" onClick={onClose} aria-label="Close">
            Close
          </button>
        </div>

        {loading ? (
          <p className="subtle">Loading…</p>
        ) : (
          <>
            <div className="grid-2" style={{ marginTop: 12 }}>
              <div className="field">
                <label htmlFor="oauth-client-id">Client ID</label>
                <input
                  id="oauth-client-id"
                  value={draft.client_id}
                  onChange={(e) => setDraft((d) => ({ ...d, client_id: e.target.value }))}
                  autoComplete="off"
                />
              </div>
              <div className="field">
                <label htmlFor="oauth-client-secret">Client secret</label>
                <input
                  id="oauth-client-secret"
                  type="password"
                  value={draft.client_secret}
                  onChange={(e) => setDraft((d) => ({ ...d, client_secret: e.target.value }))}
                  placeholder={hasSecret ? "Leave blank to keep the current secret" : ""}
                  autoComplete="new-password"
                />
              </div>
            </div>

            <div className="field">
              <label htmlFor="oauth-redirect">Callback URL</label>
              <input
                id="oauth-redirect"
                value={draft.redirect_url}
                onChange={(e) => setDraft((d) => ({ ...d, redirect_url: e.target.value }))}
                placeholder="Leave blank to use this control plane's public URL"
                autoComplete="off"
              />
              <p className="field-hint">
                Register this exact URL as the app&apos;s callback in {label}&apos;s OAuth app settings.
              </p>
            </div>

            {provider === "gitlab" && (
              <div className="field">
                <label htmlFor="oauth-base">GitLab base URL</label>
                <input
                  id="oauth-base"
                  value={draft.base_url}
                  onChange={(e) => setDraft((d) => ({ ...d, base_url: e.target.value }))}
                  placeholder="https://gitlab.com (or your self-hosted instance)"
                  autoComplete="off"
                />
              </div>
            )}

            {error && <p className="form-error">{error}</p>}

            <div className="row" style={{ marginTop: 14, justifyContent: "flex-end", gap: 8 }}>
              <button type="button" className="button secondary sm" onClick={onClose} disabled={busy}>
                Cancel
              </button>
              <button
                type="button"
                className="button sm"
                disabled={busy || !oauthAppSaveReady(draft.client_id, draft.client_secret, hasSecret)}
                onClick={() => void save()}
              >
                {busy ? "Saving…" : `Save and connect ${label}`}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
