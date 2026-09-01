"use client";

import { useState } from "react";

type Kind = "webhook" | "slack" | "email";

const STATUSES = ["failed", "timed_out", "canceled", "skipped"];

/**
 * Create a notification target.
 *
 * The form shows only the fields the chosen channel actually uses, because a form that
 * asks for an SMTP host and a webhook URL at the same time teaches nobody which one
 * matters. Scoping (labels, statuses) is shared across channels and stays visible.
 */
export function TargetForm({ onDone, onCancel }: { onDone: () => void; onCancel: () => void }) {
  const [kind, setKind] = useState<Kind>("slack");
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [cfg, setCfg] = useState<Record<string, string>>({});
  const [labels, setLabels] = useState("");
  const [statuses, setStatuses] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = (k: string, v: string) => setCfg((c) => ({ ...c, [k]: v }));

  async function submit() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/notification-targets", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          kind,
          url: url.trim(),
          config: cfg,
          server_labels: parseLabels(labels),
          on_statuses: statuses,
        }),
      });
      const body = await res.json().catch(() => null);
      if (!res.ok) throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      onDone();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="panel stack" style={{ gap: 12 }}>
      <div className="seg">
        {(["slack", "email", "webhook"] as Kind[]).map((k) => (
          <button key={k} type="button" className={`seg-btn ${kind === k ? "on" : ""}`} onClick={() => setKind(k)}>
            {k}
          </button>
        ))}
      </div>

      <div>
        <label htmlFor="nt-name">Name</label>
        <input id="nt-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Ops channel" />
      </div>

      {kind !== "email" && (
        <div>
          <label htmlFor="nt-url">{kind === "slack" ? "Slack incoming webhook URL" : "Endpoint URL"}</label>
          <input id="nt-url" value={url} onChange={(e) => setUrl(e.target.value)}
            placeholder={kind === "slack" ? "https://hooks.slack.com/services/..." : "https://example.com/hook"} />
          {kind === "webhook" && (
            <>
              <label htmlFor="nt-auth" style={{ marginTop: 8 }}>Authorization header (optional)</label>
              <input id="nt-auth" value={cfg.auth_header ?? ""} onChange={(e) => set("auth_header", e.target.value)}
                placeholder="Bearer ..." />
            </>
          )}
        </div>
      )}

      {kind === "email" && (
        <>
          <div className="row" style={{ gap: 10 }}>
            <div style={{ flex: 2 }}>
              <label htmlFor="nt-host">SMTP host</label>
              <input id="nt-host" value={cfg.smtp_host ?? ""} onChange={(e) => set("smtp_host", e.target.value)} placeholder="smtp.example.com" />
            </div>
            <div style={{ flex: 1 }}>
              <label htmlFor="nt-port">Port</label>
              <input id="nt-port" value={cfg.smtp_port ?? ""} onChange={(e) => set("smtp_port", e.target.value)} placeholder="587" />
            </div>
          </div>
          <div className="row" style={{ gap: 10 }}>
            <div style={{ flex: 1 }}>
              <label htmlFor="nt-user">Username (optional)</label>
              <input id="nt-user" value={cfg.smtp_user ?? ""} onChange={(e) => set("smtp_user", e.target.value)} />
            </div>
            <div style={{ flex: 1 }}>
              <label htmlFor="nt-pass">Password</label>
              <input id="nt-pass" type="password" value={cfg.smtp_password ?? ""} onChange={(e) => set("smtp_password", e.target.value)} />
            </div>
          </div>
          <div>
            <label htmlFor="nt-from">From</label>
            <input id="nt-from" value={cfg.from ?? ""} onChange={(e) => set("from", e.target.value)} placeholder="croncompose@example.com" />
          </div>
          <div>
            <label htmlFor="nt-to">To</label>
            <input id="nt-to" value={cfg.to ?? ""} onChange={(e) => set("to", e.target.value)} placeholder="ops@example.com, oncall@example.com" />
          </div>
          <p className="field-hint">
            Port 465 uses implicit TLS; anything else upgrades with STARTTLS. A password is never
            sent over a connection that failed to upgrade.
          </p>
        </>
      )}

      <div>
        <label htmlFor="nt-labels">Only for servers with labels (optional)</label>
        <input id="nt-labels" value={labels} onChange={(e) => setLabels(e.target.value)} placeholder="env=prod, role=db" />
        <p className="field-hint">Leave empty to cover the whole fleet.</p>
      </div>

      <div>
        <label>Only for these outcomes (optional)</label>
        <div className="cluster" style={{ gap: 6, marginTop: 6 }}>
          {STATUSES.map((s) => (
            <button
              key={s}
              type="button"
              className={`chip ${statuses.includes(s) ? "selected" : ""}`}
              onClick={() => setStatuses((cur) => cur.includes(s) ? cur.filter((x) => x !== s) : [...cur, s])}
            >
              {s}
            </button>
          ))}
        </div>
        <p className="field-hint">Nothing selected means every non-success outcome.</p>
      </div>

      {error && <p className="form-error" style={{ margin: 0 }}>{error}</p>}

      <div className="cluster" style={{ gap: 8 }}>
        <button type="button" className="button" onClick={() => void submit()} disabled={busy || !name.trim()}>
          {busy ? "Saving..." : "Add target"}
        </button>
        <button type="button" className="button secondary" onClick={onCancel} disabled={busy}>Cancel</button>
      </div>
    </div>
  );
}

/** "env=prod, role=db" into an object. Mirrors the job wizard's label parsing. */
function parseLabels(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const part of raw.split(/[,\n]/)) {
    const [k, ...rest] = part.split("=");
    const key = k?.trim();
    const value = rest.join("=").trim();
    if (key && value) out[key] = value;
  }
  return out;
}
