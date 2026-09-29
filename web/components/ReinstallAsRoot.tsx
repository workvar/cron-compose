"use client";

import { useState } from "react";
import type { IssueTokenResponse } from "@/lib/types";
import CopyButton from "@/components/CopyButton";

// Fallback path to root: issue a fresh enrollment token and re-run the installer
// with AGENT_RUN_AS_ROOT=1, instead of relying on the live "Agent root access"
// toggle (which needs the agent to reconnect over gRPC to ever settle). Useful
// when that toggle is stuck, or when someone would rather reinstall than wait.
export function ReinstallAsRoot({ serverId }: { serverId: string }) {
  const [busy, setBusy] = useState(false);
  const [command, setCommand] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function issue() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/servers/${encodeURIComponent(serverId)}/enrollment-token`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) throw new Error(await res.text());
      const body = (await res.json()) as IssueTokenResponse;
      setCommand(body.install_command_root);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="panel" style={{ marginBottom: 18 }}>
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <span style={{ fontWeight: 700, fontSize: 15 }}>Reinstall as root</span>
          <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
            Skip the toggle above: re-run the installer on this server with the agent
            running as root from the start. This issues a new enrollment token, so the
            existing agent will re-enroll when you run the command.
          </p>
        </div>
        {!command && (
          <button type="button" className="button secondary" onClick={issue} disabled={busy}>
            {busy ? "Issuing…" : "Get reinstall command"}
          </button>
        )}
      </div>
      {error && <p className="form-error" style={{ marginTop: 8 }}>{error}</p>}
      {command && (
        <div className="code-block" style={{ marginTop: 12 }}>
          <pre className="review-script" style={{ marginTop: 0 }}>{command}</pre>
          <CopyButton value={command} label="Copy" />
        </div>
      )}
    </div>
  );
}
