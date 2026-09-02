"use client";

import { useEffect, useState } from "react";
import type { ListResponse, Secret } from "@/lib/types";
import { SearchableSelect } from "@/components/SearchableSelect";
import { IconKey } from "@/components/icons";

export default function SecretsPage() {
  const [items, setItems] = useState<Secret[]>([]);
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  const [scope, setScope] = useState<"global" | "server" | "job">("global");
  const [scopeID, setScopeID] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    try {
      const res = await fetch("/api/secrets").then((r) => r.json() as Promise<ListResponse<Secret>>);
      setItems(res.items);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  useEffect(() => { load(); }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/secrets", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name, value, scope, scope_id: scopeID || undefined }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setName(""); setValue(""); setScopeID("");
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    if (!confirm("Delete this secret? Jobs that reference it will lose the value on next sync.")) return;
    await fetch(`/api/secrets/${id}`, { method: "DELETE" });
    await load();
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Secrets</h1>
          <p className="subtle">
            Values are write-only. Reference a secret by name in a job&apos;s <code>secret_refs</code>; the agent
            receives it as an environment variable at run time.
          </p>
        </div>
      </div>

      <div className="panel" style={{ maxWidth: 560 }}>
        <div className="card-head"><div className="card-title">Add a secret</div></div>
        <form onSubmit={create}>
          <div className="grid-2">
            <div className="field">
              <label>Name (env var)</label>
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder="API_KEY" required />
            </div>
            <div className="field">
              <label htmlFor="secret-scope">Scope</label>
              <SearchableSelect
                id="secret-scope"
                value={scope}
                onChange={(v) => setScope(v as typeof scope)}
                options={[
                  { value: "global", label: "global" },
                  { value: "server", label: "server" },
                  { value: "job", label: "job" },
                ]}
              />
            </div>
          </div>
          {scope !== "global" && (
            <div className="field">
              <label>Scope ID ({scope})</label>
              <input value={scopeID} onChange={(e) => setScopeID(e.target.value)} placeholder={`${scope}_id`} required />
            </div>
          )}
          <div className="field">
            <label>Value</label>
            <input type="password" value={value} onChange={(e) => setValue(e.target.value)} required />
          </div>
          {error && <div className="form-error" style={{ marginBottom: 14 }}>{error}</div>}
          <button type="submit" className="button" disabled={busy || !name || !value}>
            {busy ? "Adding…" : "Add secret"}
          </button>
        </form>
      </div>

      <h2>Existing secrets</h2>
      {items.length === 0 ? (
        <div className="panel"><div className="empty">No secrets yet.</div></div>
      ) : (
        <div className="stack">
          {items.map((s) => (
            <div key={s.id} className="panel">
              <div className="row">
                <div className="cluster" style={{ flexWrap: "nowrap" }}>
                  <span className="mini-icon"><IconKey /></span>
                  <div>
                    <div style={{ fontWeight: 700, color: "var(--text)" }}>{s.name}</div>
                    <div className="subtle" style={{ fontSize: 12 }}>
                      scope: {s.scope}{s.scope_id ? ` (${s.scope_id.slice(0, 8)})` : ""} · created {new Date(s.created_at).toLocaleString()}
                    </div>
                  </div>
                </div>
                <button className="button danger sm" onClick={() => remove(s.id)}>Delete</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}
