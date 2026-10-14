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

{
  // Split install/build waves must not report Build: Skipped after a real go build.
  const goApps: ProgressApp[] = [
    {
      name: "Proxy",
      root: "proxy",
      language: "go",
      install: "go mod download && go build -o proxy .",
      run: "./proxy",
      processManager: "pm2",
    },
  ];
  const goFlow = [
    [{ app: "Proxy", phase: "install" }],
    [{ app: "Proxy", phase: "build" }],
    [{ app: "Proxy", phase: "run" }],
  ];
  const logs = [
    { chunk: "clone: tmp clone ready (abc)\n" },
    { chunk: "install: flow wave 1/3: Proxy·install\n" },
    { chunk: "install: go mod download (in /opt/releases/r1/proxy)\n" },
    { chunk: "install: done (in /opt/releases/r1/proxy)\n" },
    { chunk: "install: flow wave 2/3: Proxy·build\n" },
    { chunk: "install: go build -o proxy . (in /opt/releases/r1/proxy)\n" },
    { chunk: "install: done (in /opt/releases/r1/proxy)\n" },
    { chunk: "release: current -> r1\n" },
    { chunk: "install: flow wave 3/3: Proxy·run\n" },
  ];
  const view = buildDeployLive({
    status: "running",
    apps: goApps,
    logs: logs.map((l, i) => ({ seq: i + 1, chunk: l.chunk })),
  });
  const waves = buildDeployWaves({ flow: goFlow, view, logs, status: "running" });
  assert.equal(waves[0].tasks[0].state, "done", "install wave");
  assert.equal(waves[1].tasks[0].state, "done", "build wave must not be skipped");
}

console.log("deploy-waves: ok");
