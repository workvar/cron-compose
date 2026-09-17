"use client";

import { useState } from "react";
import { beginUpdating } from "@/lib/updating";
import { useAgentUpdateProgress } from "@/lib/useAgentUpdateProgress";
import { UpdateProgressMeter } from "@/components/UpdateProgressMeter";

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
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stackStarted, setStackStarted] = useState(false);

  // Non-stack agent updates get their own narrated, refresh-proof progress.
  // Stack updates hand off to the full-screen UpdatingOverlay instead, since
  // the whole control-plane host goes down while it rebuilds itself.
  const { phase, detail, percent, elapsed, start } = useAgentUpdateProgress(
    serverId,
    targetVersion,
    !stack && updateAvailable,
  );

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
      if (stack) {
        setStackStarted(true);
        beginUpdating(targetVersion!, { stack: true, serverIds: [serverId] });
      } else {
        start();
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const title = stack ? "Stack update available" : "Agent update available";
  const detailCopy = stack
    ? "This host will git-checkout the release, rebuild web + control plane + agent, then restart."
    : "This agent will clone the release tag, rebuild itself from source, and restart.";

  const active = phase !== "idle" && phase !== "timeout" && phase !== "failed";
  const buttonLabel = busy
    ? "Updating…"
    : stack
      ? stackStarted
        ? "Started"
        : "Update"
      : phase === "done"
        ? "Updated"
        : phase === "timeout" || phase === "failed"
          ? "Retry"
          : active
            ? (
                <span className="cluster" style={{ gap: 6, flexWrap: "nowrap" }}>
                  <span className="agent-spinner" aria-hidden />
                  {phase === "restarting" || phase === "stopping" ? "Restarting…" : "Updating…"}
                </span>
              )
            : "Update";

  const disabled = !canUpdate || busy || stackStarted || active;

  return (
    <div className="panel" style={{ marginBottom: 18 }}>
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 700, color: "var(--text)" }}>{title}</div>
          <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
            {currentVersion ? `Running ${currentVersion}. ` : ""}
            Version {targetVersion} is available. {detailCopy}
            {stack && stackStarted && " Update started."}
          </p>
          {!stack && active && phase !== "done" && (
            <UpdateProgressMeter
              stack={false}
              phase={phase}
              detail={detail}
              percent={percent}
              elapsedMs={elapsed}
            />
          )}
          {!stack && phase === "done" && (
            <p className="subtle" style={{ margin: "8px 0 0", fontSize: 13, color: "var(--ok, #1c8a4f)" }}>
              Update complete — refreshing…
            </p>
          )}
          {!stack && phase === "timeout" && (
            <p className="form-error" style={{ margin: "8px 0 0", fontSize: 13 }}>
              This is taking longer than expected. Check the agent's update log on the server, then refresh.
            </p>
          )}
          {!stack && phase === "failed" && (
            <p className="form-error" style={{ margin: "8px 0 0", fontSize: 13 }}>
              {detail || "The update failed. Check the agent's log on the server, then retry."}
            </p>
          )}
          {error && <p className="form-error" style={{ marginTop: 8 }}>{error}</p>}
        </div>
        <button type="button" className="button sm" disabled={disabled} onClick={() => void update()}>
          {buttonLabel}
        </button>
      </div>
      {!canUpdate && !stackStarted && !active && (
        <p className="subtle" style={{ fontSize: 12, marginTop: 10 }}>
          The agent must be online to receive the update.
        </p>
      )}
    </div>
  );
}
