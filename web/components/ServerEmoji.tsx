"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { SERVER_EMOJI_OPTIONS, serverEmoji, withServerEmoji } from "@/lib/server-emoji";
import type { Server } from "@/lib/types";

/** Green monochrome emoji badge used as a server identifier. */
export function ServerEmojiBadge({
  emoji,
  size = "md",
  title,
}: {
  emoji: string;
  size?: "sm" | "md" | "lg";
  title?: string;
}) {
  return (
    <span className={`server-emoji server-emoji-${size}`} title={title} aria-hidden={!title}>
      <span className="server-emoji-glyph">{emoji || "🖥️"}</span>
    </span>
  );
}

type PickerProps = {
  server: Server;
  onSaved?: (server: Server) => void;
};

/** Assigns a green-monochrome emoji identifier stored in server.labels.emoji. */
export function ServerEmojiPicker({ server, onSaved }: PickerProps) {
  const router = useRouter();
  const [emoji, setEmoji] = useState(serverEmoji(server));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  async function save(next: string) {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/servers/${server.id}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ labels: withServerEmoji(server.labels, next) }),
      });
      if (!res.ok) throw new Error(await res.text());
      const updated = (await res.json()) as Server;
      setEmoji(serverEmoji(updated));
      setOpen(false);
      onSaved?.(updated);
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="server-emoji-picker">
      <button
        type="button"
        className="server-emoji-trigger"
        disabled={busy}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-label="Choose server emoji"
        title="Choose a green emoji identifier"
      >
        <ServerEmojiBadge emoji={emoji || "🖥️"} size="lg" />
        <span className="subtle" style={{ fontSize: 13 }}>
          {emoji ? "Change emoji" : "Assign emoji"}
        </span>
      </button>
      {open && (
        <div className="server-emoji-panel" role="listbox" aria-label="Server emoji">
          <button
            type="button"
            className={`server-emoji-option${!emoji ? " on" : ""}`}
            onClick={() => void save("")}
            disabled={busy}
          >
            <ServerEmojiBadge emoji="🖥️" size="md" />
          </button>
          {SERVER_EMOJI_OPTIONS.map((opt) => (
            <button
              key={opt}
              type="button"
              className={`server-emoji-option${emoji === opt ? " on" : ""}`}
              onClick={() => void save(opt)}
              disabled={busy}
              aria-label={`Set emoji ${opt}`}
            >
              <ServerEmojiBadge emoji={opt} size="md" />
            </button>
          ))}
          {error && <p className="form-error" style={{ gridColumn: "1 / -1", margin: 0 }}>{error}</p>}
        </div>
      )}
    </div>
  );
}
