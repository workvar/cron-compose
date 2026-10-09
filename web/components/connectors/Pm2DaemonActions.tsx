"use client";

import { useConnectorCommand } from "./useConnectorCommand";
import { StepList } from "./StepList";

/**
 * Daemon-wide pm2 controls: save the process list and install the boot hook.
 * These are not tied to a single process ref.
 */
export function Pm2DaemonActions({
  connectorId,
  enabled,
}: {
  connectorId: string;
  enabled: boolean;
}) {
  const { busy, error, result, send } = useConnectorCommand();

  async function run(action: "save" | "startup") {
    if (action === "startup") {
      const ok = window.confirm(
        "Run pm2 startup? On some hosts this prints a sudo command you may need to run once.",
      );
      if (!ok) return;
    }
    await send(`/api/connectors/${encodeURIComponent(connectorId)}/actions`, {
      method: "POST",
      body: JSON.stringify({ action, ref: "" }),
    });
  }

  return (
    <div className="stack" style={{ gap: 8 }}>
      <div className="cluster" style={{ gap: 6, flexWrap: "wrap" }}>
        <button
          type="button"
          className="button secondary sm"
          disabled={busy || !enabled}
          onClick={() => void run("save")}
          title="pm2 save — persist the current process list for reboot"
        >
          pm2 save
        </button>
        <button
          type="button"
          className="button secondary sm"
          disabled={busy || !enabled}
          onClick={() => void run("startup")}
          title="pm2 startup — install the boot resurrection hook"
        >
          pm2 startup
        </button>
        <span className="subtle" style={{ fontSize: 12 }}>
          Save freezes the process list; startup makes it come back after reboot.
        </span>
      </div>
      {error && <p className="form-error" style={{ margin: 0 }}>{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"} style={{ margin: 0 }}>
          <strong>{result.status}</strong>
          {result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
    </div>
  );
}
