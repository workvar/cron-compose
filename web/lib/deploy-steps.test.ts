import { buildDeploySteps } from "./deploy-steps.ts";
import { emptyAdvanced } from "./deploy-spec.ts";
import type { ProjectBlock } from "./project-blocks.ts";

function block(partial: Partial<ProjectBlock> & { id: string }): ProjectBlock {
  return {
    name: "app",
    root: ".",
    language: "node",
    install: "npm ci && npm run build",
    port: "3000",
    processManager: "pm2",
    autoDetect: false,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
    ...partial,
  };
}

const base = {
  repo: "acme/shop",
  branch: "main",
  serverName: "prod-1",
  clonePath: "/opt/apps/shop",
  advanced: emptyAdvanced,
};

let failed = 0;
function assert(cond: unknown, msg: string) {
  if (!cond) {
    failed++;
    console.error("FAIL:", msg);
  }
}

{
  const steps = buildDeploySteps({
    ...base,
    blocks: [block({ id: "b1", healthPath: "/healthz", healthTimeout: "30" })],
    advanced: { ...emptyAdvanced, autoRollback: true },
  });
  const ids = steps.map((s) => s.id);
  assert(ids[0] === "preflight", "starts with preflight");
  assert(ids.includes("clone"), "has clone");
  assert(ids.includes("install-b1"), "has install");
  assert(ids.includes("release"), "has release");
  assert(ids.includes("start-b1"), "has start");
  assert(ids.includes("health-b1"), "has per-app health");
  assert(ids.includes("rollback"), "has auto-rollback");
  assert(steps.find((s) => s.id === "clone")?.detail?.includes("acme/shop @ main"), "clone mentions repo/branch");
}

{
  const steps = buildDeploySteps({
    ...base,
    blocks: [
      block({ id: "web", name: "web", root: "apps/web", processManager: "pm2" }),
      block({ id: "api", name: "api", root: "apps/api", install: "go build ./...", processManager: "systemd", port: "8080" }),
    ],
  });
  assert(steps.filter((s) => s.id.startsWith("install-")).length === 2, "one install per app");
  assert(steps.filter((s) => s.id.startsWith("start-")).length === 2, "one start per app");
}

if (failed) {
  console.error(`${failed} assertion(s) failed`);
  process.exit(1);
}
console.log("ok");
