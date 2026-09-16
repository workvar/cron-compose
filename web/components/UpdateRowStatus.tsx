"use client";

import { useEffect, useRef } from "react";
import { useAgentUpdateProgress } from "@/lib/useAgentUpdateProgress";

type Props = {
  serverId: string;
  targetVersion?: string;
  stack: boolean;
  /** True once the update POST for this row has resolved 200 OK. */
  started: boolean;
  busyLabel: boolean;
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

  if (phase === "building" || phase === "restarting") {
    return (
      <span className="cluster" style={{ gap: 6, flexWrap: "nowrap", justifyContent: "flex-end" }}>
        <span className="agent-spinner" aria-hidden />
        {phase === "restarting" ? "Restarting…" : "Building…"}
      </span>
    );
  }
  if (phase === "done") return <>Updated</>;
  if (phase === "timeout") return <>Retry</>;
  return <>Update</>;
}
