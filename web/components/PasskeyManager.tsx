"use client";

import { useEffect, useRef, useState } from "react";
import { IconKey, IconPlus, IconTrash } from "@/components/icons";
import type { Passkey } from "@/lib/types";
import { deletePasskey, normalizePasskeyName, registerPasskey, renamePasskey } from "@/lib/webauthn";

function usedLabel(p: Passkey): string {
  if (p.last_used_at) return `Last used ${new Date(p.last_used_at).toLocaleString()}`;
  return `Added ${new Date(p.created_at).toLocaleString()}`;
}

function isCancelled(err: unknown): boolean {
  return err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "AbortError");
}

export function PasskeyManager({ initial }: { initial: Passkey[] }) {
  const [items, setItems] = useState(initial);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const renameInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editingId) renameInputRef.current?.focus();
  }, [editingId]);

  async function add() {
    const labeled = normalizePasskeyName(name);
    setBusy(true);
    setError(null);
    try {
      const created = await registerPasskey(labeled);
      setItems((prev) => [created, ...prev]);
      setName("");
    } catch (err) {
      if (isCancelled(err)) return;
      setError((err as Error).message || "Could not add passkey");
    } finally {
      setBusy(false);
    }
  }

  function startRename(p: Passkey) {
    if (busy) return;
    setError(null);
    setEditingId(p.id);
    setDraft(p.name || "Passkey");
  }

  function cancelRename() {
    setEditingId(null);
    setDraft("");
  }

  async function saveRename(p: Passkey) {
    const labeled = normalizePasskeyName(draft);
    if (labeled === (p.name || "Passkey")) {
      cancelRename();
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const updated = await renamePasskey(p.id, labeled);
      setItems((prev) => prev.map((item) => (item.id === p.id ? updated : item)));
      cancelRename();
    } catch (err) {
      setError((err as Error).message || "Could not rename passkey");
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
      if (editingId === p.id) cancelRename();
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
            <div className="cluster" style={{ flexWrap: "nowrap", flex: 1, minWidth: 0 }}>
              <span className="mini-icon"><IconKey /></span>
              <div style={{ flex: 1, minWidth: 0 }}>
                {editingId === p.id ? (
                  <div className="cluster" style={{ gap: 8, flexWrap: "wrap" }}>
                    <input
                      ref={renameInputRef}
                      value={draft}
                      onChange={(e) => setDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          void saveRename(p);
                        } else if (e.key === "Escape") {
                          e.preventDefault();
                          cancelRename();
                        }
                      }}
                      disabled={busy}
                      aria-label={`Rename passkey ${p.name}`}
                      style={{ maxWidth: 240 }}
                    />
                    <button type="button" className="button sm" disabled={busy} onClick={() => void saveRename(p)}>
                      Save
                    </button>
                    <button type="button" className="button secondary sm" disabled={busy} onClick={cancelRename}>
                      Cancel
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => startRename(p)}
                    disabled={busy}
                    style={{
                      fontWeight: 700,
                      color: "var(--text)",
                      background: "none",
                      border: 0,
                      padding: 0,
                      cursor: busy ? "default" : "text",
                      textAlign: "left",
                    }}
                    title="Click to rename"
                  >
                    {p.name || "Passkey"}
                  </button>
                )}
                <div className="subtle" style={{ fontSize: 12 }}>{usedLabel(p)}</div>
              </div>
            </div>
            <button
              type="button"
              className="button icon-danger"
              disabled={busy}
              onClick={() => void remove(p)}
              aria-label={busy ? "Deleting passkey" : `Delete passkey ${p.name || "Passkey"}`}
              title="Delete passkey"
            >
              <IconTrash />
            </button>
          </div>
        </div>
      ))}

      {error && <p className="form-error">{error}</p>}

      <div>
        <label htmlFor="passkey-name">Name</label>
        <input
          id="passkey-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Passkey"
          autoComplete="off"
          disabled={busy}
        />
      </div>
      <div>
        <button type="button" className="button" disabled={busy} onClick={() => void add()}>
          <IconPlus /> {busy ? "Waiting for passkey…" : "Add passkey"}
        </button>
      </div>
    </div>
  );
}
