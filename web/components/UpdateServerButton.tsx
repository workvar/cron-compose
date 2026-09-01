"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

type Props = {
  serverId: string;
  currentVersion?: string;
  targetVersion?: string;
  canUpdate: boolean;
  updateAvailable: boolean;
};

export function UpdateServerButton({
  serverId,
  currentVersion,
  targetVersion,
  canUpdate,
  updateAvailable,
}: Props) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  if (!updateAvailable || !targetVersion) return null;

  async function update() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/v1/servers/${serverId}/update`, { method: "POST" });
      const body = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
      if (!res.ok) {
        throw new Error(body?.error?.message ?? `Update failed (${res.status})`);
      }
      setDone(true);
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="panel" style={{ marginBottom: 18 }}>
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div>
          <div style={{ fontWeight: 700, color: "var(--text)" }}>Agent update available</div>
          <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
            {currentVersion ? `Running ${currentVersion}. ` : ""}
            Version {targetVersion} is available.
            {done && " Update offered — the agent will restart when it applies the new binary."}
          </p>
          {error && <p className="form-error" style={{ marginTop: 8 }}>{error}</p>}
        </div>
        <button
          type="button"
          className="button sm"
          disabled={!canUpdate || busy || done}
          onClick={() => void update()}
        >
          {busy ? "Updating…" : done ? "Update offered" : "Update agent"}
        </button>
      </div>
      {!canUpdate && !done && (
        <p className="subtle" style={{ fontSize: 12, marginTop: 10 }}>
          The agent must be online to receive the update.
        </p>
      )}
    </div>
  );
}
