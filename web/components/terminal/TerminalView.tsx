"use client";

// Live terminal: renders xterm.js and bridges it to the control-plane WebSocket. Output
// frames are binary (raw PTY bytes); control frames (started/exit/error) are JSON text.
// Input and resize are sent as JSON text frames. See docs/terminal.md for the protocol.
import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

export type TermStatus = "connecting" | "open" | "closed" | "error";

type Props = {
  serverId: string;
  mode: "shell" | "command";
  command?: string;
  /** OS user to run the session as. Empty or absent means the agent's own user. */
  runAs?: string;
  onClose: () => void;
};

const dim = (s: string) => `\r\n\x1b[90m${s}\x1b[0m\r\n`;
const red = (s: string) => `\r\n\x1b[31m${s}\x1b[0m\r\n`;

export default function TerminalView({ serverId, mode, command, runAs, onClose }: Props) {
  const holder = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const [status, setStatus] = useState<TermStatus>("connecting");

  useEffect(() => {
    const el = holder.current;
    if (!el) return;

    const term = new Terminal({
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      fontSize: 13,
      cursorBlink: true,
      theme: { background: "#0b1220", foreground: "#dbe5d8" },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(el);
    fit.fit();

    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/api/servers/${serverId}/terminal/ws`);
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      setStatus("open");
      ws.send(JSON.stringify({
        type: "init",
        mode,
        command: command ?? "",
        run_as: runAs ?? "",
        cols: term.cols,
        rows: term.rows,
      }));
      term.focus();
    };

    ws.onmessage = (ev) => {
      if (typeof ev.data !== "string") {
        term.write(new Uint8Array(ev.data as ArrayBuffer));
        return;
      }
      try {
        const e = JSON.parse(ev.data) as { type: string; code?: number; message?: string };
        if (e.type === "exit") {
          term.write(dim(`[process exited${e.code !== undefined ? ` with code ${e.code}` : ""}]`));
        } else if (e.type === "error") {
          term.write(red(`[error: ${e.message ?? "unknown"}]`));
          setStatus("error");
        }
      } catch {
        /* ignore malformed control frame */
      }
    };

    ws.onclose = () => setStatus((s) => (s === "error" ? s : "closed"));
    ws.onerror = () => setStatus("error");

    const data = term.onData((d) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "stdin", data: d }));
    });

    const sendResize = () => {
      try {
        fit.fit();
      } catch {
        /* container not measurable yet */
      }
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };
    const ro = new ResizeObserver(sendResize);
    ro.observe(el);
    window.addEventListener("resize", sendResize);

    return () => {
      ro.disconnect();
      window.removeEventListener("resize", sendResize);
      data.dispose();
      try {
        ws.close();
      } catch {
        /* already closed */
      }
      term.dispose();
    };
  }, [serverId, mode, command, runAs]);

  const ended = status === "closed" || status === "error";

  return (
    <div className="term-wrap">
      <div className="term-bar">
        <span className={`status ${statusTone(status)}`}>{statusLabel(status)}</span>
        <div className="cluster">
          {ended ? (
            <button className="button secondary sm" type="button" onClick={() => closeRef.current()}>
              New session
            </button>
          ) : (
            <button className="button danger sm" type="button" onClick={() => closeRef.current()}>
              Disconnect
            </button>
          )}
        </div>
      </div>
      <div ref={holder} className="term-holder" />
    </div>
  );
}

function statusTone(s: TermStatus): string {
  if (s === "open") return "ok";
  if (s === "error") return "danger";
  if (s === "closed") return "neutral";
  return "info";
}

function statusLabel(s: TermStatus): string {
  if (s === "open") return "connected";
  if (s === "error") return "error";
  if (s === "closed") return "disconnected";
  return "connecting…";
}
