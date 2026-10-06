import Link from "next/link";
import type { Server } from "@/lib/types";
import { ServerEmojiBadge } from "./ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";

const tone: Record<Server["status"], string> = {
  online: "ok",
  offline: "danger",
  pending: "neutral",
};

export function ServerCard({ server }: { server: Server }) {
  const emoji = serverEmojiOrFallback(server);
  const otherLabels = Object.entries(server.labels || {}).filter(([k]) => k !== "emoji");

  return (
    <Link href={`/servers/${server.id}`} className="panel">
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div className="cluster" style={{ flexWrap: "nowrap" }}>
          <ServerEmojiBadge emoji={emoji} size="md" />
          <div>
            <div style={{ fontWeight: 700, fontSize: 15, color: "var(--text)" }}>{server.name}</div>
            <div className="subtle" style={{ fontSize: 12 }}>
              {server.os || "unknown"} / {server.arch || "unknown"}
            </div>
          </div>
        </div>
        <span className={`status ${tone[server.status]}`}>{server.status}</span>
      </div>
      {server.description && (
        <p className="subtle" style={{ margin: "12px 0 0", fontSize: 13 }}>{server.description}</p>
      )}
      <div className="faint" style={{ fontSize: 12, marginTop: 12 }}>
        {otherLabels.length > 0
          ? otherLabels.map(([k, v]) => `${k}=${v}`).join(" · ")
          : server.agent_version
            ? `agent ${server.agent_version}`
            : "no labels"}
      </div>
    </Link>
  );
}
