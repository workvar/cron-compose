import assert from "node:assert/strict";
import { visibleTerminalUsers } from "./terminal-users.ts";
import {
  ControlPlaneError,
  agentRootView,
  applyToggleFailure,
  parseControlPlaneError,
} from "./agent-root.ts";

{
  assert.deepEqual(agentRootView({ enabled: false, euidRoot: false, busy: false }), {
    label: "Off",
    tone: "neutral",
  });
  assert.deepEqual(agentRootView({ enabled: false, euidRoot: false, busy: true }), {
    label: "Enabling",
    tone: "warn",
  });
  assert.deepEqual(agentRootView({ enabled: true, euidRoot: true, busy: false }), {
    label: "On (root)",
    tone: "ok",
  });
  assert.deepEqual(agentRootView({ enabled: true, euidRoot: false, busy: false }), {
    label: "On (waiting)",
    tone: "warn",
  });
  assert.deepEqual(agentRootView({
    enabled: true,
    euidRoot: false,
    busy: false,
    error: "sudo: a password is required",
  }), {
    label: "Error",
    tone: "danger",
  });
}

{
  const offline = new ControlPlaneError(503, "agent_offline", "agent offline");
  const enable = applyToggleFailure({
    requestedEnabled: true,
    previousEnabled: false,
    previousEuidRoot: false,
    error: offline,
  });
  assert.equal(enable.enabled, true);
  assert.equal(enable.euidRoot, false);
  assert.equal(enable.error, null);
  assert.equal(agentRootView({ ...enable, busy: false }).label, "On (waiting)");

  const disable = applyToggleFailure({
    requestedEnabled: false,
    previousEnabled: true,
    previousEuidRoot: true,
    error: offline,
  });
  assert.equal(disable.enabled, false);
  assert.equal(disable.error, null);
  assert.equal(agentRootView({ ...disable, busy: false }).label, "Off");

  const hard = applyToggleFailure({
    requestedEnabled: true,
    previousEnabled: false,
    previousEuidRoot: false,
    error: new ControlPlaneError(500, "send_failed", "boom"),
  });
  assert.equal(hard.enabled, false);
  assert.equal(hard.error, "boom");
  assert.equal(agentRootView({ ...hard, busy: false }).label, "Error");

  const hardDisable = applyToggleFailure({
    requestedEnabled: false,
    previousEnabled: true,
    previousEuidRoot: true,
    error: new ControlPlaneError(500, "send_failed", "boom"),
  });
  assert.equal(hardDisable.enabled, true);
  assert.equal(hardDisable.euidRoot, true);
}

{
  const err = parseControlPlaneError(503, JSON.stringify({
    error: { code: "agent_offline", message: "agent offline" },
  }), "fallback");
  assert.equal(err.code, "agent_offline");
  assert.equal(err.status, 503);
  assert.equal(err.message, "agent offline");
}

{
  const users = [
    { username: "a", uid: 1000, home: "", shell: "/bin/bash", available: true },
    { username: "b", uid: 1001, home: "", shell: "/bin/bash", available: false },
  ];
  const rootMode = (enabled: boolean, euidRoot: boolean) => enabled && euidRoot;
  assert.equal(visibleTerminalUsers(users, rootMode(true, false)).length, 1);
  assert.equal(visibleTerminalUsers(users, rootMode(true, true)).map((u) => u.username).join(","), "a,b");
  assert.equal(visibleTerminalUsers(users, rootMode(false, false)).length, 1);
}

console.log("ok");
