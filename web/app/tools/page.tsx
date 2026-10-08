"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { SearchableSelect } from "@/components/SearchableSelect";
import { UserSwitcher } from "@/components/terminal/UserSwitcher";
import { IconTools } from "@/components/icons";
import {
  clearToolsCache,
  mergeToolStatuses,
  readToolsCache,
  writeToolsCache,
} from "@/lib/tools-cache";
import { consumeToolsSSE, type ToolsStreamDone } from "@/lib/tools-stream";
import type { ListResponse, Server, ToolStatus } from "@/lib/types";
import "./tools.css";

const INSTALLABLE = ["node", "go", "python", "pm2", "git", "yarn", "pnpm", "bun"] as const;

const TOOL_BLURB: Record<string, string> = {
  node: "Node.js LTS via nvm (user-local)",
  go: "Go toolchain under ~/.local/go",
  python: "python3 + pip (apt/brew; root for apt)",
  pm2: "pm2 via npm -g",
  git: "git (apt/brew; root for apt)",
  yarn: "yarn via npm -g",
  pnpm: "pnpm via npm -g",
  bun: "Bun under ~/.bun",
};

type RowState = "idle" | "waiting" | "ready" | "error";
type JobOp = "install" | "uninstall";

type DockJob = {
  op: JobOp;
  tool: string;
  percent: number;
  log: string;
  done: boolean;
  failed?: boolean;
};

function pickStatus(byName: Record<string, ToolStatus>, name: string): ToolStatus | undefined {
  if (byName[name]) return byName[name];
  if (name === "python") return byName.python3;
  return undefined;
}

export default function ToolsPage() {
  const [servers, setServers] = useState<Server[]>([]);
  const [serverId, setServerId] = useState("");
  const [runAs, setRunAs] = useState("");
  const [byName, setByName] = useState<Record<string, ToolStatus>>({});
  const [rowState, setRowState] = useState<Record<string, RowState>>({});
  const [error, setError] = useState<string | null>(null);
  const [job, setJob] = useState<DockJob | null>(null);
  const [dockOpen, setDockOpen] = useState(true);
  const scanGen = useRef(0);
  const logPaneRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    let live = true;
    fetch("/api/servers")
      .then((r) => r.json() as Promise<ListResponse<Server>>)
      .then((d) => {
        if (!live) return;
        const items = d.items || [];
        setServers(items);
        if (!serverId && items[0]) setServerId(items[0].id);
      })
      .catch(() => live && setServers([]));
    return () => {
      live = false;
    };
  }, [serverId]);

  const serverOptions = useMemo(
    () => servers.map((s) => ({ value: s.id, label: `${s.name} · ${s.status}` })),
    [servers],
  );

  const startJob = useCallback((op: JobOp, tool: string) => {
    setDockOpen(true);
    setJob({
      op,
      tool,
      percent: 1,
      log: `${op === "install" ? "Installing" : "Uninstalling"} ${tool}…\n`,
      done: false,
    });
  }, []);

  const scanTool = useCallback(
    async (tool: string, gen: number) => {
      if (!serverId) return;
      setRowState((s) => ({ ...s, [tool]: "waiting" }));
      try {
        const q = new URLSearchParams({ tool });
        if (runAs) q.set("run_as", runAs);
        const res = await fetch(`/api/servers/${serverId}/tools?${q}`);
        const body = await res.json().catch(() => ({}));
        if (gen !== scanGen.current) return;
        if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
        const tools = (body.tools || []) as ToolStatus[];
        setByName((prev) => {
          const next = mergeToolStatuses(prev, tools);
          writeToolsCache(serverId, runAs, next);
          return next;
        });
        setRowState((s) => ({ ...s, [tool]: "ready" }));
      } catch (e) {
        if (gen !== scanGen.current) return;
        setRowState((s) => ({ ...s, [tool]: "error" }));
        setError((e as Error).message);
      }
    },
    [serverId, runAs],
  );

  const scanAll = useCallback(
    (force: boolean) => {
      if (!serverId) return;
      const gen = ++scanGen.current;
      setError(null);

      if (!force) {
        const cached = readToolsCache(serverId, runAs);
        if (cached) {
          setByName(cached.tools);
          const ready: Record<string, RowState> = {};
          for (const name of INSTALLABLE) ready[name] = "ready";
          setRowState(ready);
          return;
        }
      } else {
        clearToolsCache(serverId, runAs);
      }

      const waiting: Record<string, RowState> = {};
      for (const name of INSTALLABLE) waiting[name] = "waiting";
      setRowState(waiting);
      setByName({});

      // Kick each tool off independently so rows fill as probes return.
      for (const name of INSTALLABLE) {
        void scanTool(name, gen);
      }
    },
    [serverId, runAs, scanTool],
  );

  useEffect(() => {
    if (serverId) scanAll(false);
  }, [serverId, runAs, scanAll]);

  useEffect(() => {
    const el = logPaneRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [job?.log]);

  async function mutate(op: JobOp, tool: string) {
    if (!serverId || job) return;
    startJob(op, tool);
    setError(null);
    try {
      const res = await fetch(`/api/servers/${serverId}/tools/${op}`, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "text/event-stream",
        },
        body: JSON.stringify({ run_as: runAs, tool }),
      });
      // Non-SSE error (offline / bad request) still returns JSON.
      const ctype = res.headers.get("content-type") || "";
      if (!res.ok && !ctype.includes("text/event-stream")) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `HTTP ${res.status}`);
      }

      let doneBody: ToolsStreamDone | null = null;

      await consumeToolsSSE(res, {
        onLog: (ev) => {
          setJob((prev) => {
            if (!prev || prev.done) return prev;
            const chunk = ev.chunk || "";
            const pct = typeof ev.percent === "number" && ev.percent > 0
              ? Math.max(prev.percent, Math.min(95, ev.percent))
              : prev.percent;
            return { ...prev, percent: pct, log: prev.log + chunk };
          });
        },
        onDone: (ev) => {
          doneBody = ev;
        },
      });

      const body: ToolsStreamDone = doneBody ?? {};
      const failed = body.status === "failed" || (body.exit_code != null && body.exit_code !== 0);
      const finalLog = body.log && body.log.length > 0 ? body.log : undefined;
      setJob((prev) => ({
        op,
        tool,
        percent: 100,
        log: finalLog || prev?.log || (failed ? `${op} failed\n` : `${op} finished\n`),
        done: true,
        failed,
      }));
      if (body.tools) {
        const tools = body.tools as ToolStatus[];
        const next: Record<string, ToolStatus> = {};
        for (const t of tools) next[t.name] = t;
        setByName(next);
        writeToolsCache(serverId, runAs, next);
        const ready: Record<string, RowState> = {};
        for (const name of INSTALLABLE) ready[name] = "ready";
        setRowState(ready);
      } else {
        clearToolsCache(serverId, runAs);
        scanAll(true);
      }
      if (failed) {
        throw new Error(body.error || `${op} failed (exit ${body.exit_code ?? "?"})`);
      }
    } catch (e) {
      setError((e as Error).message);
      setJob((prev) =>
        prev
          ? {
              ...prev,
              percent: 100,
              done: true,
              failed: true,
              log: prev.log + `\nERROR: ${(e as Error).message}\n`,
            }
          : prev,
      );
    }
  }

  const waitingCount = INSTALLABLE.filter((n) => rowState[n] === "waiting").length;
  const readyCount = INSTALLABLE.filter((n) => rowState[n] === "ready").length;

  return (
    <div className="tools-page">
      <div className="page-head">
        <div>
          <h1 style={{ display: "flex", alignItems: "center", gap: 10, margin: 0 }}>
            <IconTools /> Tools
          </h1>
          <p className="subtle" style={{ marginTop: 6 }}>
            Detect and install Node, Go, Python, pm2, and friends on a server account.
            Detection uses that user&apos;s login PATH (so nvm under <code>pi</code> shows up).
          </p>
        </div>
      </div>

      <div className="panel tools-selectors">
        <div className="grid-2">
          <div className="field">
            <label htmlFor="tools-server">Server</label>
            {servers.length === 0 ? (
              <p className="field-hint">
                No servers yet. <Link href="/servers/new">Add a server</Link> first.
              </p>
            ) : (
              <SearchableSelect
                id="tools-server"
                value={serverId}
                onChange={setServerId}
                options={serverOptions}
                placeholder="Select a server…"
                aria-label="Server"
              />
            )}
          </div>
          <div className="field">
            <label htmlFor="tools-user">Account</label>
            {serverId ? (
              <UserSwitcher id="tools-user" serverId={serverId} value={runAs} onChange={setRunAs} />
            ) : (
              <p className="field-hint">Pick a server first.</p>
            )}
            <p className="field-hint">Installs land in this account&apos;s home when possible.</p>
          </div>
        </div>
        <div className="cluster" style={{ marginTop: 8, justifyContent: "space-between" }}>
          <p className="subtle" style={{ margin: 0, fontSize: 13 }}>
            {waitingCount > 0
              ? `Loaded ${readyCount}/${INSTALLABLE.length} · waiting for ${waitingCount}…`
              : readyCount > 0
                ? `${readyCount} tools ready (cached until install/uninstall)`
                : "Pick a server to scan"}
          </p>
          <button
            type="button"
            className="button secondary sm"
            onClick={() => scanAll(true)}
            disabled={!serverId || !!job || waitingCount > 0}
          >
            {waitingCount > 0 ? "Scanning…" : "Rescan"}
          </button>
        </div>
      </div>

      {error && <div className="form-error" style={{ marginTop: 16 }}>{error}</div>}

      <div className="panel" style={{ marginTop: 16 }}>
        <div className="card-title">Toolchains</div>
        <div className="tools-grid">
          {INSTALLABLE.map((name) => {
            const st = pickStatus(byName, name);
            const state = rowState[name] || "idle";
            const installed = !!(st?.installed || (name === "python" && byName.python3?.installed));
            const version = st?.version || (name === "python" ? byName.python3?.version : "") || "";
            const path = st?.path || "";
            const active = job && !job.done && job.tool === name;
            const pct = active ? Math.round(job.percent) : 0;

            return (
              <div key={name} className="tools-row">
                <div className="tools-row-meta">
                  <div className="cluster" style={{ gap: 8, alignItems: "center" }}>
                    <strong>{name}</strong>
                    {state === "waiting" || state === "idle" ? (
                      <span className="pill neutral">checking…</span>
                    ) : (
                      <span className={`pill ${installed ? "ok" : "neutral"}`}>
                        {installed ? "installed" : "missing"}
                      </span>
                    )}
                  </div>
                  <p className="field-hint" style={{ margin: "4px 0 0" }}>
                    {TOOL_BLURB[name]}
                    {version ? <> · {version}</> : null}
                    {path ? <> · <code>{path}</code></> : null}
                    {state === "waiting" ? (
                      <span className="tools-status-waiting"> · waiting…</span>
                    ) : null}
                  </p>
                </div>
                <div className="tools-row-actions">
                  {(installed || (active && job.op === "uninstall")) && (
                    <button
                      type="button"
                      className={`button secondary sm tools-progress-btn`}
                      disabled={!serverId || (!!job && !(active && job.op === "uninstall")) || state === "waiting"}
                      onClick={() => void mutate("uninstall", name)}
                    >
                      {active && job.op === "uninstall" ? (
                        <>
                          <span className="tools-progress-fill" style={{ width: `${pct}%` }} />
                          <span className="tools-progress-label">Removing {pct}%</span>
                        </>
                      ) : (
                        <span className="tools-progress-label">Uninstall</span>
                      )}
                    </button>
                  )}
                  <button
                    type="button"
                    className="button sm tools-progress-btn"
                    disabled={!serverId || (!!job && !(active && job.op === "install")) || state === "waiting"}
                    onClick={() => void mutate("install", name)}
                  >
                    {active && job.op === "install" ? (
                      <>
                        <span className="tools-progress-fill" style={{ width: `${pct}%` }} />
                        <span className="tools-progress-label">Installing {pct}%</span>
                      </>
                    ) : (
                      <span className="tools-progress-label">
                        {installed ? "Reinstall" : "Install"}
                      </span>
                    )}
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {job && (
        <div className={`tools-dock ${dockOpen ? "expanded" : "collapsed"}`} role="region" aria-label="Install log">
          <button
            type="button"
            className="tools-dock-tab"
            onClick={() => setDockOpen((v) => !v)}
            aria-expanded={dockOpen}
          >
            <span className="tools-dock-title">
              {job.op === "uninstall" ? "Uninstall" : "Install"} · {job.tool}
            </span>
            <span className="tools-dock-sub">
              {job.done
                ? job.failed
                  ? "Failed"
                  : "Done"
                : `${Math.round(job.percent)}%`}
            </span>
            <span className="tools-dock-chev">{dockOpen ? "▾" : "▴"}</span>
          </button>
          {dockOpen && (
            <div className="tools-dock-body" ref={logPaneRef}>
              {job.log || "Waiting for output…"}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
