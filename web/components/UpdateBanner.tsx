"use client";

import Link from "next/link";
import type { UpdateStatus } from "@/lib/types";

export function UpdateBanner({ status }: { status: UpdateStatus }) {
  const pending = status.items.filter((s) => s.update_available);
  if (!status.latest_version || pending.length === 0) return null;

  const online = pending.filter((s) => s.can_update).length;

  return (
    <div className="setup-banner" role="status">
      <div className="setup-banner-head">
        <span className="setup-banner-dot" style={{ background: "var(--warn)" }} />
        <div>
          <strong>Agent update available — {status.latest_version}</strong>
          <p>
            {pending.length} server{pending.length === 1 ? "" : "s"} running an older agent.
            {online > 0
              ? ` ${online} can be updated now from Settings.`
              : " Connect agents to apply the update."}
          </p>
          <div className="setup-banner-actions">
            <Link href="/settings#updates" className="button sm">Update agents</Link>
            <Link href="/servers" className="button ghost sm">Review servers</Link>
            {status.release_url && (
              <a href={status.release_url} className="button ghost sm" target="_blank" rel="noreferrer">
                Release notes
              </a>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
