"use client";

import { useCallback, useEffect, useState } from "react";
import { IconBell } from "@/components/icons";
import {
  PUSH_EVENT_LABELS,
  deletePushSubscription,
  listPushSubscriptions,
  pushSupported,
  subscribeBrowserPush,
  testPush,
  updatePushEvents,
  type PushSubscriptionRow,
} from "@/lib/web-push";

const ALL_EVENTS = Object.keys(PUSH_EVENT_LABELS);

/**
 * Per-device browser push: works with the tab closed via the service worker.
 * Independent of admin Slack/email notification targets.
 */
export function PushNotifications() {
  const [supported, setSupported] = useState(false);
  const [perm, setPerm] = useState<NotificationPermission | "unknown">("unknown");
  const [items, setItems] = useState<PushSubscriptionRow[]>([]);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<{ ok: boolean; text: string } | null>(null);
  const [events, setEvents] = useState<string[]>(ALL_EVENTS);

  const refresh = useCallback(async () => {
    const rows = await listPushSubscriptions();
    setItems(rows);
    if (rows[0]?.events?.length) setEvents(rows[0].events);
  }, []);

  useEffect(() => {
    const ok = pushSupported();
    setSupported(ok);
    if (!ok) return;
    setPerm(Notification.permission);
    void refresh();
  }, [refresh]);

  async function enable() {
    setBusy(true);
    setNote(null);
    try {
      await subscribeBrowserPush(events);
      setPerm(Notification.permission);
      await refresh();
      setNote({ ok: true, text: "This device will get push notifications." });
    } catch (e) {
      setNote({ ok: false, text: (e as Error).message });
    } finally {
      setBusy(false);
    }
  }

  async function disable(id: string) {
    setBusy(true);
    setNote(null);
    try {
      await deletePushSubscription(id);
      await refresh();
      setNote({ ok: true, text: "Push disabled on this device." });
    } catch (e) {
      setNote({ ok: false, text: (e as Error).message });
    } finally {
      setBusy(false);
    }
  }

  async function saveEvents(id: string) {
    setBusy(true);
    setNote(null);
    try {
      await updatePushEvents(id, events);
      await refresh();
      setNote({ ok: true, text: "Event preferences saved." });
    } catch (e) {
      setNote({ ok: false, text: (e as Error).message });
    } finally {
      setBusy(false);
    }
  }

  async function sendTest() {
    setBusy(true);
    setNote(null);
    try {
      await testPush();
      setNote({ ok: true, text: "Test sent — check your notification center." });
    } catch (e) {
      setNote({ ok: false, text: (e as Error).message });
    } finally {
      setBusy(false);
    }
  }

  if (!supported) {
    return (
      <div className="panel">
        <div className="empty">
          This browser does not support Web Push (needs HTTPS or localhost, and a modern browser).
        </div>
      </div>
    );
  }

  const active = items[0];

  return (
    <div className="panel">
      <div className="row">
        <div className="cluster" style={{ flexWrap: "nowrap" }}>
          <span className="mini-icon"><IconBell /></span>
          <div>
            <div style={{ fontWeight: 700, color: "var(--text)" }}>Browser push</div>
            <div className="subtle" style={{ fontSize: 12 }}>
              Deploy start / success / fail, job failures, and updates — even with the tab closed.
              {perm !== "unknown" ? ` · Permission: ${perm}` : ""}
            </div>
          </div>
        </div>
        <div className="cluster">
          {active ? (
            <>
              <button type="button" className="button secondary sm" disabled={busy} onClick={() => void sendTest()}>
                Test
              </button>
              <button type="button" className="button secondary sm" disabled={busy} onClick={() => void disable(active.id)}>
                Disable
              </button>
            </>
          ) : (
            <button type="button" className="button sm" disabled={busy} onClick={() => void enable()}>
              {busy ? "Enabling…" : "Enable on this device"}
            </button>
          )}
        </div>
      </div>

      <div style={{ marginTop: 14, display: "grid", gap: 8 }}>
        {ALL_EVENTS.map((ev) => {
          const on = events.includes(ev);
          return (
            <label key={ev} className="cluster" style={{ gap: 10, cursor: "pointer" }}>
              <input
                type="checkbox"
                checked={on}
                disabled={busy}
                onChange={() => {
                  setEvents((prev) =>
                    on ? prev.filter((e) => e !== ev) : [...prev, ev],
                  );
                }}
              />
              <span style={{ fontSize: 13.5 }}>{PUSH_EVENT_LABELS[ev] || ev}</span>
            </label>
          );
        })}
      </div>

      {active && (
        <div className="cluster" style={{ marginTop: 12 }}>
          <button
            type="button"
            className="button secondary sm"
            disabled={busy}
            onClick={() => void saveEvents(active.id)}
          >
            Save event preferences
          </button>
          <span className="subtle" style={{ fontSize: 12 }}>
            Applies to this browser subscription.
          </span>
        </div>
      )}

      {note && (
        <p className={note.ok ? "subtle" : "form-error"} style={{ marginTop: 12, marginBottom: 0 }}>
          {note.text}
        </p>
      )}
    </div>
  );
}
