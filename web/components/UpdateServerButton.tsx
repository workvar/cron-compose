"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { beginUpdating } from "@/lib/updating";

type Props = {
  serverId: string;
  currentVersion?: string;
  targetVersion?: string;
  canUpdate: boolean;
  updateAvailable: boolean;
  stack?: boolean;
};

export function UpdateServerButton({
  serverId,
  currentVersion,
  targetVersion,
  canUpdate,
  updateAvailable,
  stack = false,
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
      const res = await fetch(`/api/servers/${serverId}/update`, { method: "POST" });
      const body = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
      if (!res.ok) {
        throw new Error(body?.error?.message ?? `Update failed (${res.status})`);
      }
      setDone(true);
      if (stack) {
        beginUpdating(targetVersion!, { stack: true, serverIds: [serverId] });
      } else {
        router.refresh();
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const title = stack ? "Stack update available" : "Agent update available";
  const detail = stack
    ? "This host will git-checkout the release, rebuild web + control plane + agent, then restart."
    : "This agent will clone the release tag, rebuild itself from source, and restart.";

  return (
    <div className="panel" style={{ marginBottom: 18 }}>
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div>
          <div style={{ fontWeight: 700, color: "var(--text)" }}>{title}</div>
          <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
            {currentVersion ? `Running ${currentVersion}. ` : ""}
            Version {targetVersion} is available. {detail}
            {done && !stack && " Update started."}
          </p>
          {error && <p className="form-error" style={{ marginTop: 8 }}>{error}</p>}
        </div>
        <button
          type="button"
          className="button sm"
          disabled={!canUpdate || busy || done}
          onClick={() => void update()}
        >
          {busy ? "Updating…" : done ? "Started" : "Update"}
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
