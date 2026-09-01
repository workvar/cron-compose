"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { NotificationTarget } from "@/lib/types";
import { IconBell, IconPlus } from "@/components/icons";
import { TargetForm } from "./TargetForm";

/**
 * The notification targets list with per-target enable, test, and delete.
 *
 * "Test" is the important control here. Every other setting in this app fails visibly
 * the moment it is wrong; a notification channel fails silently, at the exact moment
 * you needed it. Sending a real message through the real channel on demand is the only
 * way to know it works.
 */
export function TargetManager({ initial }: { initial: NotificationTarget[] }) {
  const router = useRouter();
  const [adding, setAdding] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [note, setNote] = useState<{ id: string; ok: boolean; text: string } | null>(null);

  async function toggle(t: NotificationTarget) {
    setBusyId(t.id);
    try {
      await fetch(`/api/notification-targets/${t.id}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ enabled: !t.enabled }),
      });
      router.refresh();
    } finally {
      setBusyId(null);
    }
  }

  async function test(t: NotificationTarget) {
    setBusyId(t.id);
    setNote(null);
    try {
      const res = await fetch(`/api/notification-targets/${t.id}/test`, { method: "POST" });
      const body = await res.json().catch(() => null);
      if (body?.delivered) {
        setNote({ id: t.id, ok: true, text: "Sent. Check the channel." });
      } else {
        setNote({ id: t.id, ok: false, text: body?.error ?? `HTTP ${res.status}` });
      }
      router.refresh();
    } finally {
      setBusyId(null);
    }
  }

  async function remove(t: NotificationTarget) {
    if (!window.confirm(`Delete the notification target "${t.name}"?`)) return;
    setBusyId(t.id);
    try {
      await fetch(`/api/notification-targets/${t.id}`, { method: "DELETE" });
      router.refresh();
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="stack">
      {initial.length === 0 && !adding && (
        <div className="panel"><div className="empty">No notification targets yet. Nothing will be sent when a job fails.</div></div>
      )}

      {initial.map((t) => (
        <div key={t.id} className="panel">
          <div className="row">
            <div className="cluster" style={{ flexWrap: "nowrap" }}>
              <span className="mini-icon"><IconBell /></span>
              <div>
                <div style={{ fontWeight: 700, color: "var(--text)" }}>{t.name}</div>
                <div className="subtle mono" style={{ fontSize: 12 }}>
                  {t.kind}{t.url ? ` · ${t.url}` : ""}
                  {t.kind === "email" && t.config?.to ? ` · ${t.config.to}` : ""}
                </div>
                <TargetScope target={t} />
              </div>
            </div>
            <div className="cluster" style={{ gap: 6 }}>
              <span className={`status ${t.enabled ? "ok" : "neutral"}`}>{t.enabled ? "enabled" : "disabled"}</span>
              <button type="button" className="button secondary sm" disabled={busyId === t.id} onClick={() => void test(t)}>Test</button>
              <button type="button" className="button secondary sm" disabled={busyId === t.id} onClick={() => void toggle(t)}>
                {t.enabled ? "Disable" : "Enable"}
              </button>
              <button type="button" className="button danger sm" disabled={busyId === t.id} onClick={() => void remove(t)}>Delete</button>
            </div>
          </div>

          {note?.id === t.id && (
            <p className={note.ok ? "subtle" : "form-error"} style={{ marginTop: 10, marginBottom: 0 }}>{note.text}</p>
          )}
          {t.last_error && note?.id !== t.id && (
            <p className="form-error" style={{ marginTop: 10, marginBottom: 0 }}>
              Last delivery failed: {t.last_error}
            </p>
          )}
        </div>
      ))}

      {adding ? (
        <TargetForm onDone={() => { setAdding(false); router.refresh(); }} onCancel={() => setAdding(false)} />
      ) : (
        <div>
          <button type="button" className="button" onClick={() => setAdding(true)}>
            <IconPlus /> Add target
          </button>
        </div>
      )}
    </div>
  );
}

/** A one-line summary of what this target listens for, so scoping is visible at rest. */
function TargetScope({ target }: { target: NotificationTarget }) {
  const labels = Object.entries(target.server_labels ?? {});
  const statuses = target.on_statuses ?? [];
  if (labels.length === 0 && statuses.length === 0) {
    return <div className="faint" style={{ fontSize: 12 }}>every failed run, every server</div>;
  }
  return (
    <div className="faint" style={{ fontSize: 12 }}>
      {statuses.length > 0 ? statuses.join(", ") : "any failure"}
      {labels.length > 0 ? ` · servers where ${labels.map(([k, v]) => `${k}=${v}`).join(", ")}` : " · every server"}
    </div>
  );
}
