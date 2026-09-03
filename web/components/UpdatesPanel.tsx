"use client";

import { useState } from "react";
import Link from "next/link";
import type { UpdateStatus } from "@/lib/types";
import { summarizeUpdates } from "@/lib/update-helpers";
import { beginUpdating } from "@/lib/updating";
import { IconArrowUpRight, IconDownload, IconServer } from "@/components/icons";

type Props = {
  initial: UpdateStatus | null;
  canUpdate: boolean;
};

export function UpdatesPanel({ initial, canUpdate }: Props) {
  const [status, setStatus] = useState(initial);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [offered, setDone] = useState<Record<string, boolean>>({});
  const view = summarizeUpdates(status);

  async function readJson(res: Response) {
    return (await res.json().catch(() => null)) as
      | (UpdateStatus & { error?: { message?: string } })
      | { error?: { message?: string } }
      | null;
  }

  async function checkNow() {
    setBusy("check");
    setError(null);
    try {
      const res = await fetch("/api/updates/check", { method: "POST" });
      const body = await readJson(res);
      if (!res.ok) {
        throw new Error(body && "error" in body ? body.error?.message ?? `Check failed (${res.status})` : `Check failed (${res.status})`);
      }
      setStatus(body as UpdateStatus);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  async function updateServer(id: string, stack: boolean) {
    const res = await fetch(`/api/servers/${id}/update`, { method: "POST" });
    const body = await readJson(res);
    if (!res.ok) {
      throw new Error(body && "error" in body ? body.error?.message ?? `Update failed (${res.status})` : `Update failed (${res.status})`);
    }
    setDone((d) => ({ ...d, [id]: true }));
    if (stack && view.latest) {
      beginUpdating(view.latest, { stack: true, serverIds: [id] });
    }
  }

  async function updateOne(id: string, stack: boolean) {
    setBusy(id);
    setError(null);
    try {
      await updateServer(id, stack);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  async function updateAll() {
    setBusy("all");
    setError(null);
    try {
      const stackIds: string[] = [];
      for (const s of view.updatable) {
        if (offered[s.server_id]) continue;
        await updateServer(s.server_id, false);
        if (s.stack) stackIds.push(s.server_id);
      }
      if (stackIds.length > 0 && view.latest) {
        beginUpdating(view.latest, { stack: true, serverIds: stackIds });
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="panel" id="updates">
      <div className="card-head" style={{ alignItems: "flex-start" }}>
        <div>
          <p className="subtle" style={{ margin: 0, fontSize: 13 }}>
            GitHub releases — each host builds from the tag
            {view.repo ? <> · <span className="mono">{view.repo}</span></> : null}
          </p>
        </div>
        <div className="cluster">
          {view.releaseUrl && (
            <a href={view.releaseUrl} className="button ghost sm" target="_blank" rel="noreferrer">
              Release notes <IconArrowUpRight />
            </a>
          )}
          {canUpdate && (
            <button type="button" className="button secondary sm" disabled={busy !== null} onClick={() => void checkNow()}>
              {busy === "check" ? "Checking…" : "Check now"}
            </button>
          )}
        </div>
      </div>

      <div className="updates-hero">
        <div>
          <div className="updates-version">{view.latest ?? "—"}</div>
          <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>{view.headline}</p>
        </div>
        {canUpdate && view.updatable.length > 0 && (
          <button
            type="button"
            className="button sm"
            disabled={busy !== null}
            onClick={() => void updateAll()}
          >
            <IconDownload />
            {busy === "all" ? "Updating…" : `Update ${view.updatable.length}`}
          </button>
        )}
      </div>

      {view.checkError && (
        <p className="form-error" style={{ marginTop: 14 }}>{view.checkError}</p>
      )}
      {error && <p className="form-error" style={{ marginTop: 14 }}>{error}</p>}

      {status && status.items.length === 0 ? (
        <div className="empty" style={{ marginTop: 16 }}>No servers enrolled yet. Add a server to update it.</div>
      ) : status && status.items.length > 0 ? (
        <div className="table-wrap" style={{ marginTop: 16 }}>
          <table className="table">
            <thead>
              <tr>
                <th>Server</th>
                <th>Running</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {status.items.map((s) => {
                const tone = s.status === "online" ? "ok" : s.status === "offline" ? "danger" : "neutral";
                const done = offered[s.server_id];
                const showUpdate = canUpdate && s.update_available;
                return (
                  <tr key={s.server_id}>
                    <td>
                      <Link href={`/servers/${s.server_id}`} className="cluster" style={{ color: "inherit", flexWrap: "nowrap" }}>
                        <span className="mini-icon"><IconServer /></span>
                        <span style={{ fontWeight: 600 }}>{s.server_name}</span>
                        {s.stack ? <span className="pill" style={{ fontSize: 11 }}>stack</span> : null}
                      </Link>
                    </td>
                    <td className="mono">{s.current_version || "unknown"}</td>
                    <td><span className={`status ${tone}`}>{s.status}</span></td>
                    <td style={{ textAlign: "right" }}>
                      {showUpdate ? (
                        <button
                          type="button"
                          className="button sm"
                          disabled={!s.can_update || busy !== null || done}
                          onClick={() => void updateOne(s.server_id, !!s.stack)}
                        >
                          {busy === s.server_id ? "Updating…" : done ? "Started" : "Update"}
                        </button>
                      ) : (
                        <span className="subtle" style={{ fontSize: 12 }}>
                          {s.current_version ? "current" : "no version"}
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="empty" style={{ marginTop: 16 }}>Could not load update status.</div>
      )}
    </div>
  );
}
