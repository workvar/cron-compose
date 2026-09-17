"use client";

import { useState } from "react";
import { IconKey, IconPlus } from "@/components/icons";
import type { Passkey } from "@/lib/types";
import { deletePasskey, registerPasskey } from "@/lib/webauthn";

function usedLabel(p: Passkey): string {
  if (p.last_used_at) return `Last used ${new Date(p.last_used_at).toLocaleString()}`;
  return `Added ${new Date(p.created_at).toLocaleString()}`;
}

function isCancelled(err: unknown): boolean {
  return err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "AbortError");
}

export function PasskeyManager({ initial }: { initial: Passkey[] }) {
  const [items, setItems] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function add() {
    setBusy(true);
    setError(null);
    try {
      const created = await registerPasskey();
      setItems((prev) => [created, ...prev]);
    } catch (err) {
      if (isCancelled(err)) return;
      setError((err as Error).message || "Could not add passkey");
    } finally {
      setBusy(false);
    }
  }

  async function remove(p: Passkey) {
    if (!window.confirm(`Delete the passkey "${p.name}"?`)) return;
    setBusy(true);
    setError(null);
    try {
      await deletePasskey(p.id);
      setItems((prev) => prev.filter((item) => item.id !== p.id));
    } catch (err) {
      setError((err as Error).message || "Could not delete passkey");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="stack">
      {items.length === 0 && (
        <div className="panel">
          <div className="empty">No passkeys yet. Add one to sign in without a password.</div>
        </div>
      )}

      {items.map((p) => (
        <div className="panel" key={p.id}>
          <div className="row">
            <div className="cluster" style={{ flexWrap: "nowrap" }}>
              <span className="mini-icon"><IconKey /></span>
              <div>
                <div style={{ fontWeight: 700, color: "var(--text)" }}>{p.name || "Passkey"}</div>
                <div className="subtle" style={{ fontSize: 12 }}>{usedLabel(p)}</div>
              </div>
            </div>
            <button
              type="button"
              className="button danger sm"
              disabled={busy}
              onClick={() => void remove(p)}
            >
              Delete
            </button>
          </div>
        </div>
      ))}

      {error && <p className="form-error">{error}</p>}

      <div>
        <button type="button" className="button" disabled={busy} onClick={() => void add()}>
          <IconPlus /> {busy ? "Waiting for passkey…" : "Add passkey"}
        </button>
      </div>
    </div>
  );
}
