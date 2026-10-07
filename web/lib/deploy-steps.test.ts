import {
  buildDeployPlan,
  buildDeploySteps,
  classifyInstallCommand,
  splitInstallCommands,
} from "./deploy-steps.ts";
import { emptyAdvanced } from "./deploy-spec.ts";
import type { ProjectBlock } from "./project-blocks.ts";

function block(partial: Partial<ProjectBlock> & { id: string }): ProjectBlock {
  return {
    name: "app",
    root: ".",
    language: "node",
    install: "npm ci && npm run build",
    run: "npm start",
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
  const parts = splitInstallCommands("npm ci && npm run build");
  assert(parts.length === 2, "split &&");
  assert(classifyInstallCommand("npm ci", "node").kind === "install", "npm ci is install");
  assert(classifyInstallCommand("npm run build", "node").kind === "build", "npm run build is build");
  assert(classifyInstallCommand("go build ./...", "go").kind === "build", "go build is build");
}

{
  const plan = buildDeployPlan({
    ...base,
    blocks: [block({ id: "b1", healthPath: "/healthz", healthTimeout: "30" })],
    advanced: { ...emptyAdvanced, autoRollback: true },
    appEnv: { app: [{ key: "API_URL", value: "https://x", sensitive: false, has_value: true }] },
  });
  const kinds = plan.map((b) => b.kind);
  assert(kinds[0] === "setup", "starts with setup");
  assert(kinds.includes("app"), "has app block");
  assert(kinds.includes("release"), "has release");
  assert(kinds.includes("policy"), "has policy for rollback");

  const app = plan.find((b) => b.kind === "app")!;
  assert(app.subtitle?.includes("repository root"), "app shows working dir");
  const titles = app.steps.map((s) => s.title);
  assert(titles.some((t) => /npm packages/i.test(t)), "install packages substep");
  assert(titles.some((t) => t === "Build"), "build substep");

  const runBlock = plan.find((b) => b.id.startsWith("run-"))!;
  const runTitles = runBlock.steps.map((s) => s.title);
  assert(runTitles.some((t) => /environment/i.test(t)), "env substep");
  assert(runTitles.some((t) => /PM2/i.test(t)), "start substep");
  assert(runTitles.some((t) => /Health/i.test(t)), "health substep");

  const withShared = buildDeployPlan({
    ...base,
    blocks: [block({ id: "b1" })],
    globalEnv: [{ key: "NODE_ENV", value: "production", sensitive: false }],
    appEnv: { app: [{ key: "API_URL", value: "https://x", sensitive: false }] },
  });
  const envDetail = withShared
    .find((b) => b.id.startsWith("run-"))!
    .steps.find((s) => s.kind === "env")!.detail!;
  assert(/2 configured/.test(envDetail), "counts shared + process env");
  assert(/1 shared/.test(envDetail), "mentions shared count");

  const releaseIdx = plan.findIndex((b) => b.kind === "release");
  const runIdx = plan.findIndex((b) => b.id.startsWith("run-"));
  assert(releaseIdx > 0 && runIdx > releaseIdx, "run blocks come after release swap");

  const steps = buildDeploySteps({
    ...base,
    blocks: [block({ id: "b1", healthPath: "/healthz", healthTimeout: "30" })],
    advanced: { ...emptyAdvanced, autoRollback: true },
  });
  assert(steps.find((s) => s.id === "clone")?.detail?.includes("acme/shop @ main"), "clone mentions repo/branch");
  assert(steps.some((s) => s.id === "rollback"), "flat list includes rollback");
}

{
  const plan = buildDeployPlan({
    ...base,
    blocks: [
      block({ id: "web", name: "web", root: "apps/web", processManager: "pm2" }),
      block({
        id: "api",
        name: "api",
        root: "apps/api",
        language: "go",
        install: "go mod download && go build -o api ./cmd/api",
        processManager: "systemd",
        port: "8080",
      }),
    ],
  });
  const apps = plan.filter((b) => b.kind === "app");
  assert(apps.length === 4, "build+run block per app");
  assert(apps[0].subtitle?.includes("apps/web"), "web root in subtitle");
  assert(apps[1].subtitle?.includes("apps/api"), "api root in subtitle");
  const apiBuild = apps.find((b) => b.id === "build-api")!;
  const apiTitles = apiBuild.steps.map((s) => s.title);
  assert(apiTitles.some((t) => /Go modules/i.test(t)), "go modules install");
  assert(apiTitles.some((t) => /Go binary/i.test(t)), "go build");
}

if (failed) {
  console.error(`${failed} assertion(s) failed`);
  process.exit(1);
}
console.log("ok");
