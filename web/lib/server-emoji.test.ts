import assert from "node:assert/strict";
import {
  SERVER_EMOJI_LABEL,
  serverEmoji,
  serverEmojiOrFallback,
  withServerEmoji,
} from "./server-emoji.ts";

assert.equal(serverEmoji({ labels: {} }), "");
assert.equal(serverEmoji({ labels: { [SERVER_EMOJI_LABEL]: "🌱" } }), "🌱");
assert.equal(serverEmojiOrFallback({ labels: {}, name: "box" }), "🖥️");
assert.deepEqual(withServerEmoji({ env: "prod" }, "🍀"), { env: "prod", emoji: "🍀" });
assert.deepEqual(withServerEmoji({ emoji: "🍀", env: "prod" }, ""), { env: "prod" });

console.log("server-emoji ok");
