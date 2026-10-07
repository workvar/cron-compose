"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { SearchableSelect } from "@/components/SearchableSelect";
import { UserSwitcher } from "@/components/terminal/UserSwitcher";
import { IconTools } from "@/components/icons";
import type { ListResponse, Server } from "@/lib/types";

type ToolStatus = {
  name: string;
  installed: boolean;
  version?: string;
  path?: string;
};

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

export default function ToolsPage() {
  const [servers, setServers] = useState<Server[]>([]);
  const [serverId, setServerId] = useState("");
  const [runAs, setRunAs] = useState("");
  const [tools, setTools] = useState<ToolStatus[] | null>(null);
  const [busy, setBusy] = useState<"detect" | string | null>(null);
  const [log, setLog] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  const detect = useCallback(async () => {
    if (!serverId) return;
    setBusy("detect");
    setError(null);
    setLog(null);
    try {
      const q = new URLSearchParams();
      if (runAs) q.set("run_as", runAs);
      const res = await fetch(`/api/servers/${serverId}/tools?${q}`);
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
      setTools((body.tools || []) as ToolStatus[]);
    } catch (e) {
      setError((e as Error).message);
      setTools(null);
    } finally {
      setBusy(null);
    }
  }, [serverId, runAs]);

  useEffect(() => {
    if (serverId) void detect();
  }, [serverId, runAs, detect]);

  async function install(tool: string) {
    if (!serverId) return;
    setBusy(tool);
    setError(null);
    setLog(null);
    try {
      const res = await fetch(`/api/servers/${serverId}/tools/install`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ run_as: runAs, tool }),
      });
      const body = await res.json().catch(() => ({}));
      if (body.log) setLog(String(body.log));
      if (body.tools) setTools(body.tools as ToolStatus[]);
      if (!res.ok || body.status === "failed") {
        throw new Error(body.error || `install failed (exit ${body.exit_code ?? "?"})`);
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  const byName = useMemo(() => {
    const m = new Map<string, ToolStatus>();
    for (const t of tools || []) m.set(t.name, t);
    return m;
  }, [tools]);

  return (
    <>
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

      <div className="panel" style={{ maxWidth: 720 }}>
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
        <div className="cluster" style={{ marginTop: 8 }}>
          <button type="button" className="button secondary sm" onClick={() => void detect()} disabled={!serverId || busy === "detect"}>
            {busy === "detect" ? "Scanning…" : "Rescan"}
          </button>
        </div>
      </div>

      {error && <div className="form-error" style={{ marginTop: 16 }}>{error}</div>}

      <div className="panel" style={{ marginTop: 16 }}>
        <div className="card-title">Toolchains</div>
        {!tools && !error && <p className="subtle">Scanning…</p>}
        {tools && (
          <div className="tools-grid" style={{ display: "grid", gap: 12, marginTop: 12 }}>
            {INSTALLABLE.map((name) => {
              const st = byName.get(name) || byName.get(name === "python" ? "python3" : name);
              const installed = !!(st?.installed || (name === "python" && byName.get("python3")?.installed));
              const version = st?.version || (name === "python" ? byName.get("python3")?.version : "") || "";
              const path = st?.path || "";
              return (
                <div
                  key={name}
                  className="row"
                  style={{
                    alignItems: "center",
                    justifyContent: "space-between",
                    gap: 12,
                    padding: "10px 0",
                    borderBottom: "1px solid var(--border, #e5e5e5)",
                  }}
                >
                  <div>
                    <div className="cluster" style={{ gap: 8, alignItems: "center" }}>
                      <strong>{name}</strong>
                      <span className={`pill ${installed ? "ok" : "neutral"}`}>
                        {installed ? "installed" : "missing"}
                      </span>
                    </div>
                    <p className="field-hint" style={{ margin: "4px 0 0" }}>
                      {TOOL_BLURB[name]}
                      {version ? <> · {version}</> : null}
                      {path ? <> · <code>{path}</code></> : null}
                    </p>
                  </div>
                  <button
                    type="button"
                    className="button sm"
                    disabled={!serverId || busy !== null}
                    onClick={() => void install(name)}
                  >
                    {busy === name ? "Installing…" : installed ? "Reinstall" : "Install"}
                  </button>
                </div>
              );
            })}
          </div>
        )}
        {tools && tools.filter((t) => !INSTALLABLE.includes(t.name as typeof INSTALLABLE[number])).length > 0 && (
          <div style={{ marginTop: 20 }}>
            <div className="card-title">Also detected</div>
            <ul className="subtle" style={{ marginTop: 8 }}>
              {tools
                .filter((t) => !INSTALLABLE.includes(t.name as typeof INSTALLABLE[number]))
                .map((t) => (
                  <li key={t.name}>
                    <code>{t.name}</code>
                    {t.installed ? ` — ${t.version || "ok"}` : " — missing"}
                    {t.path ? ` (${t.path})` : ""}
                  </li>
                ))}
            </ul>
          </div>
        )}
      </div>

      {log && (
        <div className="panel" style={{ marginTop: 16 }}>
          <div className="card-title">Install log</div>
          <pre
            style={{
              marginTop: 8,
              maxHeight: 320,
              overflow: "auto",
              fontSize: 12,
              whiteSpace: "pre-wrap",
            }}
          >
            {log}
          </pre>
        </div>
      )}
    </>
  );
}
