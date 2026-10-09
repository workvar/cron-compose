"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { IconSearch } from "@/components/icons";
import { ServerEmojiBadge } from "@/components/ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";
import type { Job, Server } from "@/lib/types";

const tone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

type Counts = { total: number; enabled: number; disabled: number; label: number };

function labelsMatch(job: Job, server: Server): boolean {
  const wanted = job.target_labels || {};
  const have = server.labels || {};
  return Object.entries(wanted).every(([k, v]) => have[k] === v);
}

function countsFor(server: Server, jobs: Job[]): Counts {
  const c = { total: 0, enabled: 0, disabled: 0, label: 0 };
  for (const job of jobs) {
    const onServer =
      (job.target_kind === "server" && job.server_id === server.id) ||
      (job.target_kind === "labels" && labelsMatch(job, server));
    if (!onServer) continue;
    c.total += 1;
    if (job.enabled) c.enabled += 1;
    else c.disabled += 1;
    if (job.target_kind === "labels") c.label += 1;
  }
  return c;
}

type Props = {
  servers: Server[];
  jobs: Job[];
};

/** Big server cards for the Jobs landing page — same shape as Deploy. */
export function JobServerCards({ servers, jobs }: Props) {
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
          No servers yet. <Link href="/servers/new">Add a server</Link>, then come back to schedule jobs.
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="search" style={{ marginBottom: 18, width: "100%", maxWidth: "none" }}>
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
            const counts = countsFor(s, jobs);
            return (
              <Link
                key={s.id}
                href={`/jobs/servers/${s.id}`}
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
                  <span><strong>{counts.total}</strong> {counts.total === 1 ? "job" : "jobs"}</span>
                  <span className="pill">On {counts.enabled}</span>
                  <span className="pill">Off {counts.disabled}</span>
                  {counts.label > 0 && <span className="pill">Labels {counts.label}</span>}
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
