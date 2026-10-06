import type { Server } from "./types";

/** Label key used to store a server's emoji identifier. */
export const SERVER_EMOJI_LABEL = "emoji";

/** Curated green-friendly emoji identifiers for servers. */
export const SERVER_EMOJI_OPTIONS = [
  "🌱", "🍀", "🌿", "🌲", "🌳", "🌴", "🌵", "🪴", "🎋", "🍃",
  "🥝", "🥒", "🫑", "🟢", "💚", "♻️", "✳️", "❇️", "🟩", "🐲",
  "🐝", "🐢", "🐸", "🐍", "🦖", "🦎", "🐛", "🦋", "🐉", "🧿",
] as const;

export function serverEmoji(server: Pick<Server, "labels"> | null | undefined): string {
  const raw = server?.labels?.[SERVER_EMOJI_LABEL]?.trim();
  return raw || "";
}

export function withServerEmoji(
  labels: Record<string, string> | null | undefined,
  emoji: string,
): Record<string, string> {
  const next = { ...(labels || {}) };
  const trimmed = emoji.trim();
  if (!trimmed) delete next[SERVER_EMOJI_LABEL];
  else next[SERVER_EMOJI_LABEL] = trimmed;
  return next;
}

/** Fallback glyph when no emoji is assigned (shown with the same green treatment). */
export function serverEmojiOrFallback(server: Pick<Server, "labels" | "name">): string {
  return serverEmoji(server) || "🖥️";
}
