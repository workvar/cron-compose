import Link from "next/link";
import { ServerEmojiBadge } from "@/components/ServerEmoji";
import { serverEmojiOrFallback } from "@/lib/server-emoji";
import type { Server } from "@/lib/types";

/** Sticky chip showing which server the current deploy (or job) targets. */
export function DeployServerChip({
  server,
  onChange,
  label = "Deploying to",
}: {
  server: Server | null | undefined;
  onChange?: () => void;
  label?: string;
}) {
  if (!server) {
    return (
      <div className="deploy-server-chip muted">
        <span className="subtle">No server selected</span>
        {onChange && (
          <button type="button" className="button ghost sm" onClick={onChange}>
            Pick server
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="deploy-server-chip">
      <ServerEmojiBadge emoji={serverEmojiOrFallback(server)} size="sm" />
      <div className="deploy-server-chip-text">
        <span className="deploy-server-chip-label">{label}</span>
        <strong>{server.name}</strong>
      </div>
      <span className={`status ${server.status === "online" ? "ok" : server.status === "offline" ? "danger" : "neutral"}`}>
        {server.status}
      </span>
      {onChange ? (
        <button type="button" className="button ghost sm" onClick={onChange}>
          Change
        </button>
      ) : (
        <Link href={`/servers/${server.id}`} className="button ghost sm">
          View
        </Link>
      )}
    </div>
  );
}
