"use client";

import { use, useCallback, useEffect, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";

// xterm touches the DOM, so load the view client-side only.
const TerminalView = dynamic(() => import("@/components/terminal/TerminalView"), { ssr: false });

type Props = {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ mode?: string; command?: string; runAs?: string }>;
};

type Mode = "shell" | "command";

export default function TerminalFullPage({ params, searchParams }: Props) {
  const { id } = use(params);
  const sp = use(searchParams);
  const mode: Mode = sp.mode === "command" ? "command" : "shell";
  const command = sp.command ?? "";
  const [runAs, setRunAs] = useState(sp.runAs ?? "");
  const [session, setSession] = useState(0);
  const [left, setLeft] = useState(false);

  const syncUrl = useCallback((nextRunAs: string) => {
    const q = new URLSearchParams();
    q.set("mode", mode);
    if (mode === "command" && command) q.set("command", command);
    if (nextRunAs.trim()) q.set("runAs", nextRunAs.trim());
    // Keep the fullscreen URL in sync so a refresh preserves the selected user.
    window.history.replaceState(null, "", `?${q.toString()}`);
  }, [mode, command]);

  const switchUser = useCallback((next: string) => {
    setRunAs(next);
    syncUrl(next);
    setSession((n) => n + 1);
  }, [syncUrl]);

  const reopen = useCallback(() => {
    setLeft(false);
    setSession((n) => n + 1);
  }, []);

  const leave = useCallback(() => {
    window.close();
    // window.close only works for script-opened tabs; show a fallback if we're still here.
    setTimeout(() => setLeft(true), 150);
  }, []);

  useEffect(() => {
    document.title = mode === "command" && command
      ? `${command} · Terminal`
      : "Terminal";
  }, [mode, command]);

  if (left) {
    return (
      <div className="term-fullscreen term-fullscreen-ended">
        <p>Session closed.</p>
        <div className="cluster">
          <button type="button" className="button" onClick={reopen}>New session</button>
          <Link href={`/servers/${id}/terminal`} className="button secondary">Back to terminal</Link>
        </div>
      </div>
    );
  }

  return (
    <div className="term-fullscreen">
      <TerminalView
        key={session}
        serverId={id}
        mode={mode}
        command={mode === "command" ? command : undefined}
        runAs={runAs.trim() || undefined}
        fullscreen
        onClose={leave}
        onNewSession={reopen}
        onSwitchUser={switchUser}
      />
    </div>
  );
}
