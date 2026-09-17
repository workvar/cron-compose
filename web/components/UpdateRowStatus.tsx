"use client";

import { useEffect, useRef } from "react";
import { useAgentUpdateProgress } from "@/lib/useAgentUpdateProgress";
import { effectivePhase } from "@/lib/update-progress";

type Props = {
  serverId: string;
  targetVersion?: string;
  stack: boolean;
  /** True once the update POST for this row has resolved 200 OK. */
  started: boolean;
  busyLabel: boolean;
};

const SHORT: Record<string, string> = {
  offered: "Sending…",
  fetching: "Fetching…",
  cloning: "Cloning…",
  downloading: "Downloading…",
  building: "Building…",
  installing: "Installing…",
  migrating: "Migrating…",
  stopping: "Stopping…",
  restarting: "Restarting…",
  verifying: "Verifying…",
};

/**
 * Narrates a single row's in-flight agent update in the Updates table, and
 * survives a page refresh (see lib/agent-update.ts) so the row doesn't just
 * flip back to a bare "Update" button while the agent is still rebuilding.
 * Stack rows are left to the full-screen UpdatingOverlay, same as before.
 */
export function UpdateRowStatus({ serverId, targetVersion, stack, started, busyLabel }: Props) {
  const { phase, start } = useAgentUpdateProgress(serverId, targetVersion, !stack);
  const kicked = useRef(false);

  useEffect(() => {
    if (started && !stack && !kicked.current) {
      kicked.current = true;
      start();
    }
    if (!started) kicked.current = false;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [started, stack]);

  if (busyLabel) return <>Updating…</>;
  if (stack) return <>{started ? "Started" : "Update"}</>;

  if (phase === "done") return <>Updated</>;
  if (phase === "timeout" || phase === "failed") return <>Retry</>;
  if (phase !== "idle") {
    const shown = effectivePhase(phase, false);
    return (
      <span className="cluster" style={{ gap: 6, flexWrap: "nowrap", justifyContent: "flex-end" }}>
        <span className="agent-spinner" aria-hidden />
        {SHORT[shown] ?? "Updating…"}
      </span>
    );
  }
  return <>Update</>;
}
