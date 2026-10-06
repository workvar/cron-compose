import assert from "node:assert/strict";
import { terminalFullscreenPath } from "./terminal-href.ts";

assert.equal(
  terminalFullscreenPath("abc", { mode: "shell" }),
  "/app/servers/abc/terminal/full?mode=shell",
);
assert.equal(
  terminalFullscreenPath("abc", { mode: "command", command: "uptime -p", runAs: "root" }),
  "/app/servers/abc/terminal/full?mode=command&command=uptime+-p&runAs=root",
);
assert.equal(
  terminalFullscreenPath("abc", { mode: "shell", runAs: "  " }),
  "/app/servers/abc/terminal/full?mode=shell",
);

console.log("terminal-href ok");
