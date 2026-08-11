"use client";

import { use, useCallback, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { IconChevronLeft, IconTerminal } from "@/components/icons";

// xterm touches the DOM, so load the view client-side only.
const TerminalView = dynamic(() => import("@/components/terminal/TerminalView"), { ssr: false });

type Props = { params: Promise<{ id: string }> };
type Mode = "shell" | "command";

export default function TerminalPage({ params }: Props) {
  const { id } = use(params);
  const [phase, setPhase] = useState<"setup" | "live">("setup");
  const [mode, setMode] = useState<Mode>("shell");
  const [command, setCommand] = useState("");
  const [session, setSession] = useState(0); // bump remounts TerminalView for a fresh connection

  const start = () => {
    if (mode === "command" && !command.trim()) return;
    setSession((n) => n + 1);
    setPhase("live");
  };
  const back = useCallback(() => setPhase("setup"), []);

  return (
    <>
      <Link href={`/servers/${id}`} className="back-link"><IconChevronLeft /> Back to server</Link>
      <div className="page-head">
        <div>
          <h1>Terminal</h1>
          <p className="subtle" style={{ marginTop: 6 }}>
            Opens a shell as the agent&apos;s user on this server. Admin/owner only; every session is audited.
          </p>
        </div>
      </div>

      {phase === "setup" ? (
        <div className="panel term-setup">
          <div className="seg">
            <button type="button" className={`seg-btn ${mode === "shell" ? "on" : ""}`} onClick={() => setMode("shell")}>
              Interactive shell
            </button>
            <button type="button" className={`seg-btn ${mode === "command" ? "on" : ""}`} onClick={() => setMode("command")}>
              Run a command
            </button>
          </div>

          {mode === "command" && (
            <div>
              <label htmlFor="term-cmd">Command</label>
              <input
                id="term-cmd"
                className="term-cmd"
                placeholder="e.g. systemctl status nginx"
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") start(); }}
                autoFocus
              />
              <p className="field-hint">Runs once via the login shell, streams output, then exits.</p>
            </div>
          )}

          <div>
            <button type="button" className="button" onClick={start} disabled={mode === "command" && !command.trim()}>
              <IconTerminal /> {mode === "shell" ? "Open shell" : "Run command"}
            </button>
          </div>
        </div>
      ) : (
        <TerminalView
          key={session}
          serverId={id}
          mode={mode}
          command={mode === "command" ? command : undefined}
          onClose={back}
        />
      )}
    </>
  );
}
