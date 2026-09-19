"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import type { Server } from "@/lib/types";
import {
  AGENT_ROOT_POLL_MS,
  AGENT_ROOT_WATCH_MS,
  agentRootPending,
  agentRootView,
  applyToggleFailure,
  decideAgentRootWatch,
  visibleStoredRootError,
} from "@/lib/agent-root";
import { setAgentRoot } from "@/lib/webauthn";

function isCancelled(err: unknown): boolean {
  return err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "AbortError");
}

async function fetchServer(id: string): Promise<Server> {
  const res = await fetch(`/api/servers/${encodeURIComponent(id)}`, { credentials: "include" });
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<Server>;
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
  const [storedError, setStoredError] = useState(server.agent_root_error ?? null);
  const [busy, setBusy] = useState(false);
  const [watching, setWatching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const watchGen = useRef(0);

  useEffect(() => {
    setEnabled(server.agent_root_enabled);
    setEuidRoot(server.agent_euid_root);
    setStoredError(server.agent_root_error ?? null);
  }, [server.agent_root_enabled, server.agent_euid_root, server.agent_root_error]);

  // Resume watching after reload when the flag and euid still disagree.
  useEffect(() => {
    if (!agentRootPending(server.agent_root_enabled, server.agent_euid_root)) return;
    void watchUntilSettled();
    return () => {
      watchGen.current += 1;
    };
    // Intentionally only re-arm when switching servers; toggle() starts its own watch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [server.id]);

  async function watchUntilSettled() {
    const gen = ++watchGen.current;
    setWatching(true);
    const started = Date.now();
    try {
      while (gen === watchGen.current) {
        await new Promise((r) => window.setTimeout(r, AGENT_ROOT_POLL_MS));
        if (gen !== watchGen.current) return;
        let snap: Server;
        try {
          snap = await fetchServer(server.id);
        } catch {
          continue;
        }
        if (gen !== watchGen.current) return;
        setEnabled(snap.agent_root_enabled);
        setEuidRoot(snap.agent_euid_root);
        setStoredError(snap.agent_root_error ?? null);
        const decision = decideAgentRootWatch({
          enabled: snap.agent_root_enabled,
          euidRoot: snap.agent_euid_root,
          elapsedMs: Date.now() - started,
          timeoutMs: AGENT_ROOT_WATCH_MS,
          storedError: snap.agent_root_error,
        });
        if (decision.action === "continue") continue;
        if (decision.action === "done") {
          setError(null);
          router.refresh();
          return;
        }
        setError(decision.error);
        router.refresh();
        return;
      }
    } finally {
      if (gen === watchGen.current) setWatching(false);
    }
  }

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
    watchGen.current += 1;
    try {
      const updated = await setAgentRoot(server.id, next);
      setEnabled(updated.agent_root_enabled);
      setEuidRoot(updated.agent_euid_root);
      setStoredError(updated.agent_root_error ?? null);
      router.refresh();
      if (agentRootPending(updated.agent_root_enabled, updated.agent_euid_root)) {
        void watchUntilSettled();
      }
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
      if (nextState.error == null) {
        router.refresh();
        if (agentRootPending(nextState.enabled, nextState.euidRoot)) {
          void watchUntilSettled();
        }
      }
    } finally {
      setBusy(false);
    }
  }

  const locked = !hasPasskeys || busy || watching;
  const displayError = error || visibleStoredRootError({
    enabled,
    euidRoot,
    storedError,
  });
  const { label, tone } = agentRootView({
    enabled,
    euidRoot,
    busy: busy || watching,
    error: displayError,
  });

  return (
    <div id="agent-root-access" className="panel" style={{ marginBottom: 18 }}>
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
          {watching && !displayError && (
            <p className="subtle" style={{ margin: "8px 0 0", fontSize: 13 }}>
              Waiting for the agent to reconnect and report its privileges…
            </p>
          )}
          {displayError && <p className="form-error" style={{ marginTop: 8 }}>{displayError}</p>}
        </div>
        <span className={`status ${tone}`}>{label}</span>
      </div>
    </div>
  );
}
