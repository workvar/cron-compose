import assert from "node:assert/strict";
import type { ProjectBlock } from "./project-blocks.ts";
import {
  addEmptyWave,
  defaultFlow,
  flowOrderError,
  flowProblems,
  flowToSpec,
  reconcileFlow,
  hasEmptyWaves,
  moveTask,
  removeWave,
  splitInstallBuild,
  type FlowWave,
} from "./deploy-flow.ts";

function block(id: string, name: string, partial: Partial<ProjectBlock> = {}): ProjectBlock {
  return {
    id,
    name,
    root: name.toLowerCase(),
    language: "node",
    install: "npm ci && npm run build",
    run: "npm start",
    cleanup: "rm -rf .git",
    preStart: "",
    port: "",
    processManager: "pm2",
    autoDetect: false,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
    startGroup: 0,
    ...partial,
  };
}

{
  const split = splitInstallBuild("npm ci && npm run build", "node");
  assert.equal(split.install, "npm ci");
  assert.equal(split.build, "npm run build");
}

{
  const blocks = [block("a", "Proxy"), block("b", "Backend", { preStart: "migrate" })];
  const flow = defaultFlow(blocks);
  assert.ok(flow.length >= 3);
  const phases = flow.flatMap((w) => w.tasks.map((t) => t.phase));
  assert.ok(phases.includes("install"));
  assert.ok(phases.includes("build"));
  assert.ok(phases.includes("cleanup"));
  assert.ok(phases.includes("run"));
  assert.ok(phases.includes("start"));
  assert.ok(phases.includes("pre_start"));
  const runAt = phases.indexOf("run");
  const startAt = phases.indexOf("start");
  assert.ok(runAt >= 0 && startAt > runAt, "run comes before start");
}

{
  const blocks = [block("a", "Proxy"), block("b", "Backend")];
  let flow = defaultFlow(blocks);
  flow = addEmptyWave(flow);
  assert.equal(hasEmptyWaves(flow), true);
  assert.deepEqual(flowProblems(flow), ["Fill or delete every empty wave"]);
  const emptyId = flow[flow.length - 1].id;
  flow = removeWave(flow, emptyId);
  assert.equal(hasEmptyWaves(flow), false);
}

{
  const blocks = [block("a", "Proxy"), block("b", "Backend")];
  let flow = defaultFlow(blocks);
  const installA = flow.flatMap((w) => w.tasks).find((t) => t.blockId === "a" && t.phase === "install");
  const buildBWave = flow.find((w) => w.tasks.some((t) => t.blockId === "b" && t.phase === "build"));
  assert.ok(installA && buildBWave);
  flow = moveTask(flow, installA!.id, buildBWave!.id);
  const together = flow.find((w) =>
    w.tasks.some((t) => t.id === installA!.id) && w.tasks.some((t) => t.blockId === "b" && t.phase === "build"),
  );
  assert.ok(together, "Proxy install can sit in the same wave as Backend build");
  const spec = flowToSpec(blocks, flow);
  assert.ok(spec.some((w) => w.some((t) => t.app === "Proxy" && t.phase === "install")));
}

{
  // Start Proxy, then install Backend — allowed across apps.
  const flow: FlowWave[] = [
    {
      id: "w0",
      tasks: [
        { id: "t1", blockId: "a", phase: "install" },
        { id: "t2", blockId: "a", phase: "build" },
      ],
    },
    { id: "w1", tasks: [{ id: "t3", blockId: "a", phase: "start" }] },
    { id: "w2", tasks: [{ id: "t4", blockId: "b", phase: "install" }] },
    { id: "w3", tasks: [{ id: "t5", blockId: "b", phase: "start" }] },
  ];
  assert.equal(flowOrderError(flow), null);
}

{
  // Same wave cannot mix start with install.
  const flow: FlowWave[] = [
    {
      id: "w0",
      tasks: [
        { id: "t1", blockId: "a", phase: "start" },
        { id: "t2", blockId: "b", phase: "install" },
      ],
    },
  ];
  assert.match(flowOrderError(flow) || "", /cannot mix/i);
}

{
  // One process cannot start before its own install.
  const flow: FlowWave[] = [
    { id: "w0", tasks: [{ id: "t1", blockId: "a", phase: "start" }] },
    { id: "w1", tasks: [{ id: "t2", blockId: "a", phase: "install" }] },
  ];
  assert.match(flowOrderError(flow) || "", /cannot run before/i);
}

{
  // A saved flow from before Run existed gets the chip before Start, not after Health.
  const blocks = [block("a", "Proxy", { healthPath: "/healthz" })];
  const flow: FlowWave[] = [
    { id: "w0", tasks: [{ id: "t1", blockId: "a", phase: "install" }] },
    { id: "w1", tasks: [{ id: "t2", blockId: "a", phase: "start" }] },
    { id: "w2", tasks: [{ id: "t3", blockId: "a", phase: "health" }] },
  ];
  const next = reconcileFlow(blocks, flow);
  const flat = next.flatMap((w) => w.tasks.map((t) => t.phase));
  const runAt = flat.indexOf("run");
  const startAt = flat.indexOf("start");
  assert.ok(runAt >= 0 && runAt < startAt, flat.join(","));
  assert.equal(flowOrderError(next), null);
}

console.log("deploy-flow: ok");
