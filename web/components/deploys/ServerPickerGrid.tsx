"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { IconSearch } from "@/components/icons";
import { ServerEmojiBadge } from "@/components/ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";
import type { Server } from "@/lib/types";

const tone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

type Props = {
  servers: Server[];
  value: string;
  onChange: (serverId: string) => void;
  onContinue: () => void;
};

/** Searchable grid of servers with emoji identifiers for the new-deploy flow. */
export function ServerPickerGrid({ servers, value, onChange, onContinue }: Props) {
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
      <div className="panel config-card">
        <p className="subtle" style={{ margin: 0 }}>
          No servers yet. <Link href="/servers/new">Add a server</Link>, then come back to deploy.
        </p>
      </div>
    );
  }

  return (
    <div className="server-picker">
      <div className="search" style={{ marginBottom: 16, maxWidth: 420 }}>
        <IconSearch />
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search servers…"
          aria-label="Search servers"
          autoFocus
        />
      </div>

      {filtered.length === 0 ? (
        <div className="panel"><div className="empty">No servers match “{query}”.</div></div>
      ) : (
        <div className="server-picker-grid" role="listbox" aria-label="Servers">
          {filtered.map((s) => {
            const selected = s.id === value;
            return (
              <button
                key={s.id}
                type="button"
                role="option"
                aria-selected={selected}
                className={`server-picker-card${selected ? " selected" : ""}`}
                onClick={() => onChange(s.id)}
                onDoubleClick={() => {
                  onChange(s.id);
                  onContinue();
                }}
              >
                <ServerEmojiBadge emoji={serverEmojiOrFallback(s)} size="lg" />
                <div className="server-picker-meta">
                  <div className="server-picker-name">{s.name}</div>
                  <div className="subtle" style={{ fontSize: 12 }}>
                    {s.os || "unknown"} / {s.arch || "unknown"}
                  </div>
                </div>
                <span className={`status ${tone[s.status]}`}>{s.status}</span>
              </button>
            );
          })}
        </div>
      )}

      <div className="deploy-bar">
        <span className="subtle deploy-bar-hint">
          {value ? "Double-click a card or continue to pick a repository." : "Select a server to deploy onto."}
        </span>
        <div className="deploy-bar-right">
          <button type="button" className="button deploy-button" disabled={!value} onClick={onContinue}>
            Continue
          </button>
        </div>
      </div>
    </div>
  );
}
