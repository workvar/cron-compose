"use client";

import type { ConnectorResource } from "@/lib/types";
import { StepList } from "./StepList";
import { useConnectorCommand } from "./useConnectorCommand";

// Which verbs make sense per connector kind. Docker has no reload; pm2's enable and
// disable mean "save the boot list", which is a different enough idea that it is not
// offered inline on a row.
const ACTIONS: Record<string, string[]> = {
  systemd: ["start", "stop", "restart", "reload", "enable", "disable"],
  docker: ["start", "stop", "restart"],
  pm2: ["start", "stop", "restart", "reload"],
  nginx: ["start", "stop", "restart", "reload"],
};

const DESTRUCTIVE = new Set(["stop", "disable"]);

export function ObjectActions({
  connectorId,
  kind,
  resource,
  enabled,
}: {
  connectorId: string;
  kind: string;
  resource: ConnectorResource;
  enabled: boolean;
}) {
  const { busy, error, result, send } = useConnectorCommand();
  const actions = ACTIONS[kind] ?? ["start", "stop", "restart"];

  async function run(action: string) {
    // Stopping something is the one place a stray click has a visible cost, so it
    // asks first. Start and restart are recoverable by clicking again.
    if (DESTRUCTIVE.has(action)) {
      const okToGo = window.confirm(`${action} ${resource.name}?`);
      if (!okToGo) return;
    }
    await send(`/api/connectors/${connectorId}/actions`, {
      method: "POST",
      body: JSON.stringify({ action, ref: resource.ref }),
    });
  }

  return (
    <div className="stack" style={{ gap: 6 }}>
      <div className="cluster" style={{ gap: 6 }}>
        {actions.map((a) => (
          <button
            key={a}
            type="button"
            className={`button sm ${DESTRUCTIVE.has(a) ? "danger" : "secondary"}`}
            disabled={busy || !enabled}
            onClick={() => run(a)}
            title={enabled ? `${a} ${resource.name}` : "The agent cannot drive this connector"}
          >
            {a}
          </button>
        ))}
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
