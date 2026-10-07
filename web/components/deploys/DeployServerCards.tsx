"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { IconSearch } from "@/components/icons";
import { ServerEmojiBadge } from "@/components/ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";
import type { Connector, Server } from "@/lib/types";

const tone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

type Counts = { pm2: number; systemd: number; docker: number; total: number };

function countsFor(serverId: string, connectors: Connector[]): Counts {
  const c = { pm2: 0, systemd: 0, docker: 0, total: 0 };
  for (const conn of connectors) {
    if (conn.server_id !== serverId) continue;
    if (conn.kind === "pm2") c.pm2 += conn.object_count;
    else if (conn.kind === "systemd") c.systemd += conn.object_count;
    else if (conn.kind === "docker") c.docker += conn.object_count;
  }
  c.total = c.pm2 + c.systemd + c.docker;
  return c;
}

type Props = {
  servers: Server[];
  connectors: Connector[];
};

/** Big server cards for the Deploy landing page. */
export function DeployServerCards({ servers, connectors }: Props) {
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter((s) => {
      const hay = `${s.name} ${s.description || ""} ${s.os || ""} ${Object.values(s.labels || {}).join(" ")}`.toLowerCase();
      return hay.includes(q);
    });
  }, [servers, query]);

  if (servers.length === 0) {
    return (
      <div className="panel">
        <div className="empty">
          No servers yet. <Link href="/servers/new">Add a server</Link>, then come back to deploy.
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="search" style={{ marginBottom: 18, maxWidth: 420 }}>
        <IconSearch />
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search servers…"
          aria-label="Search servers"
        />
      </div>

      {filtered.length === 0 ? (
        <div className="panel"><div className="empty">No servers match “{query}”.</div></div>
      ) : (
        <div className="deploy-server-grid">
          {filtered.map((s) => {
            const counts = countsFor(s.id, connectors);
            return (
              <Link
                key={s.id}
                href={`/deploys/servers/${s.id}`}
                className="deploy-server-card"
              >
                <div className="deploy-server-card-top">
                  <ServerEmojiBadge emoji={serverEmojiOrFallback(s)} size="lg" />
                  <span className={`status ${tone[s.status]}`}>{s.status}</span>
                </div>
                <div className="deploy-server-card-name">{s.name}</div>
                {s.description && (
                  <div className="subtle" style={{ fontSize: 13, marginTop: 4 }}>{s.description}</div>
                )}
                <div className="subtle" style={{ fontSize: 12, marginTop: 8 }}>
                  {s.os || "unknown"} / {s.arch || "unknown"}
                </div>
                <div className="deploy-server-counts">
                  <span><strong>{counts.total}</strong> processes</span>
                  <span className="pill">PM2 {counts.pm2}</span>
                  <span className="pill">systemd {counts.systemd}</span>
                  <span className="pill">Docker {counts.docker}</span>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
