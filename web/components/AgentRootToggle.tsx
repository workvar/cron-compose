"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import type { Server } from "@/lib/types";
import { agentRootView, applyToggleFailure, visibleStoredRootError } from "@/lib/agent-root";
import { setAgentRoot } from "@/lib/webauthn";

function isCancelled(err: unknown): boolean {
  return err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "AbortError");
}

export function AgentRootToggle({
  server,
  hasPasskeys,
}: {
  server: Server;
  hasPasskeys: boolean;
}) {
  const router = useRouter();
  const [enabled, setEnabled] = useState(server.agent_root_enabled);
  const [euidRoot, setEuidRoot] = useState(server.agent_euid_root);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setEnabled(server.agent_root_enabled);
    setEuidRoot(server.agent_euid_root);
  }, [server.agent_root_enabled, server.agent_euid_root]);

  async function toggle(next: boolean) {
    if (next === enabled) return;
    const ok = window.confirm(
      next
        ? "Turn on agent root access for this server? You'll confirm with a passkey. The agent will restart as root so the terminal can open as other OS users."
        : "Turn off agent root access? You'll confirm with a passkey. The agent will demote back to its install user.",
    );
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      const updated = await setAgentRoot(server.id, next);
      setEnabled(updated.agent_root_enabled);
      setEuidRoot(updated.agent_euid_root);
      router.refresh();
    } catch (err) {
      if (isCancelled(err) || (err as Error).message === "Passkey verification was cancelled") return;
      const nextState = applyToggleFailure({
        requestedEnabled: next,
        previousEnabled: enabled,
        previousEuidRoot: euidRoot,
        error: err,
      });
      setEnabled(nextState.enabled);
      setEuidRoot(nextState.euidRoot);
      setError(nextState.error);
      if (nextState.error == null) router.refresh();
    } finally {
      setBusy(false);
    }
  }

  const locked = !hasPasskeys || busy;
  const displayError = error || visibleStoredRootError({
    enabled,
    euidRoot,
    storedError: server.agent_root_error,
  });
  const { label, tone } = agentRootView({ enabled, euidRoot, busy, error: displayError });

  return (
    <div className="panel" style={{ marginBottom: 18 }}>
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <label className="cluster" style={{ gap: 10, alignItems: "center", marginBottom: 0, color: "var(--text)" }}>
            <input
              type="checkbox"
              role="switch"
              checked={enabled}
              disabled={locked}
              onChange={(e) => void toggle(e.target.checked)}
              aria-label="Agent root access"
            />
            <span style={{ fontWeight: 700, fontSize: 15 }}>Agent root access</span>
          </label>
          <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
            Restart this agent as root so the web terminal can open as other OS users.
            Enabling or disabling requires a passkey — a password is not accepted.
          </p>
          {!hasPasskeys && (
            <p className="subtle" style={{ margin: "8px 0 0", fontSize: 13 }}>
              Enroll a passkey in <Link href="/settings">Settings → Security</Link> before turning this on.
            </p>
          )}
          {displayError && <p className="form-error" style={{ marginTop: 8 }}>{displayError}</p>}
        </div>
        <span className={`status ${tone}`}>{label}</span>
      </div>
    </div>
  );
}
