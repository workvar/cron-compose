import assert from "node:assert/strict";
import { activeWaveIndex, buildDeployWaves, currentWaveFromLogs } from "./deploy-waves.ts";
import { buildDeployLive, type ProgressApp } from "./deploy-progress.ts";

const apps: ProgressApp[] = [
  { name: "Proxy", root: "proxy", language: "go", install: "go build -o proxy .", run: "./proxy", processManager: "pm2", port: 8080 },
  { name: "Backend", root: "backend", language: "go", install: "go build -o backend .", processManager: "pm2" },
];

const flow = [
  [{ app: "Proxy", phase: "install" }, { app: "Proxy", phase: "build" }],
  [{ app: "Proxy", phase: "run" }],
  [{ app: "Backend", phase: "install" }],
];

{
  const hit = currentWaveFromLogs([
    { chunk: "install: flow wave 1/3: Proxy·install, Proxy·build\n" },
    { chunk: "install: flow wave 2/3: Proxy·run\n" },
  ]);
  assert.deepEqual(hit, { index: 2, total: 3 });
}

{
  const view = buildDeployLive({
    status: "running",
    apps,
    logs: [
      { stream: "stdout", seq: 1, chunk: "clone: tmp clone ready (abc)\n" },
      { stream: "stdout", seq: 2, chunk: "install: flow wave 2/3: Proxy·run\n" },
      { stream: "stdout", seq: 3, chunk: "install: go build -o proxy . (in /opt/apps/proxy)\n" },
      { stream: "stdout", seq: 4, chunk: "install: done (in /opt/apps/proxy)\n" },
      { stream: "stdout", seq: 5, chunk: "release: current -> abc\n" },
      { stream: "stdout", seq: 6, chunk: "run: pm2 start ecosystem.config.js --update-env (in /opt/apps/proxy)\n" },
    ],
  });
  const waves = buildDeployWaves({ flow, view, logs: [
    { chunk: "install: flow wave 2/3: Proxy·run\n" },
  ], status: "running" });
  assert.equal(waves.length, 3);
  assert.equal(waves[0].tasks[0].app, "Proxy");
  assert.equal(waves[1].tasks[0].phase, "run");
  assert.equal(waves[1].tasks[0].state, "active");
  assert.ok(activeWaveIndex(waves) >= 0);
}

console.log("deploy-waves: ok");
