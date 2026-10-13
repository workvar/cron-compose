import assert from "node:assert/strict";
import {
  applyFramework,
  blocksReady,
  blocksToDeployApps,
  ensureUniqueBlockNames,
  hasDuplicateRoots,
  nameFromRoot,
  normalizeBlockRoot,
  seedBlockFromInspect,
  seedBlocksFromInspect,
} from "./project-blocks.ts";

assert.equal(normalizeBlockRoot(""), ".");
assert.equal(normalizeBlockRoot("apps/web"), "apps/web");
assert.equal(normalizeBlockRoot("/backend"), "backend");
assert.equal(normalizeBlockRoot("./apps/web/"), "apps/web");
assert.equal(nameFromRoot(".", "acme/widgets"), "widgets");
assert.equal(nameFromRoot("apps/web", "acme/widgets"), "web");
assert.equal(nameFromRoot("/backend", "acme/widgets"), "backend");

const base = {
  language: "",
  install: "",
  run: "",
  cleanup: "",
  preStart: "",
  port: "",
  processManager: "none",
  processName: "",
  processNameAuto: false,
  ecosystemFile: "",
  autoDetect: true,
  healthPath: "",
  healthPort: "",
  healthTimeout: "",
  startGroup: 0,
} as const;

const dup = ensureUniqueBlockNames([
  { id: "1", name: "web", root: "a", ...base },
  { id: "2", name: "web", root: "b", ...base },
]);
assert.equal(dup[1].name, "web-2");

assert.equal(
  hasDuplicateRoots([
    { id: "1", name: "a", root: "apps/web", ...base },
    { id: "2", name: "b", root: "apps/web", ...base },
  ]),
  true,
);

assert.equal(
  blocksReady([{ id: "1", name: "x", root: "", ...base }]),
  false,
);

assert.equal(
  blocksReady([{ id: "1", name: "x", root: "apps/web", ...base }]),
  true,
);

{
  const block = seedBlockFromInspect(
    {
      language: "node",
      install_script: "npm ci",
      has_pm2_ecosystem: false,
      supports_port: true,
      workspaces: ["apps/web"],
      root_directory: "apps/web",
      clone_url: "https://github.com/acme/widgets.git",
      default_branch: "main",
      clone_path: "/var/croncompose/acme-widgets",
      process_manager: "pm2",
    },
    "acme/widgets",
  );
  assert.equal(block.root, "apps/web");
  assert.equal(block.name, "web");
  assert.equal(block.language, "node");
  assert.equal(block.install, "npm ci");
  assert.equal(block.run, "npm start");
  assert.equal(block.processManager, "pm2");
  assert.equal(block.port, "3000");
  assert.equal(block.processName, "");
}

{
  const blocks = seedBlocksFromInspect(
    {
      language: "go",
      install_script: "go build -o proxy .",
      has_pm2_ecosystem: true,
      supports_port: true,
      workspaces: [],
      root_directory: ".",
      clone_url: "https://github.com/acme/peepal.git",
      default_branch: "main",
      clone_path: "/opt/apps/peepal",
      process_manager: "pm2",
      pm2_apps: [
        { name: "peepal-proxy", script: "./go", args: "run .", cwd: "proxy", port: 8080 },
        { name: "peepal-api", script: "./api", cwd: "api", port: 3000 },
      ],
    },
    "acme/peepal",
  );
  assert.equal(blocks.length, 2);
  assert.equal(blocks[0].name, "peepal-proxy");
  assert.equal(blocks[0].root, "proxy");
  assert.equal(blocks[0].processName, "peepal-proxy");
  assert.equal(blocks[0].processNameAuto, true);
  assert.ok(blocks[0].ecosystemFile);
  assert.equal(blocks[0].processManager, "pm2");
  assert.equal(blocks[0].run, "./go run .");
  assert.equal(blocks[0].port, "8080");
  assert.equal(blocks[1].name, "peepal-api");
  assert.equal(blocks[1].root, "api");
  assert.equal(blocks[1].processName, "peepal-api");
  assert.equal(blocks[1].processNameAuto, true);
}

{
  const next = applyFramework(
    {
      id: "1",
      name: "web",
      root: ".",
      language: "node",
      install: "old",
      run: "old",
      cleanup: "",
      preStart: "",
      port: "",
      processManager: "none",
      processName: "",
      processNameAuto: false,
      ecosystemFile: "",
      autoDetect: true,
      healthPath: "",
      healthPort: "",
      healthTimeout: "",
      startGroup: 0,
    },
    "nextjs",
  );
  assert.equal(next.language, "nextjs");
  assert.equal(next.install, "npm ci && npm run build");
  assert.equal(next.run, "npm start");
  assert.equal(next.port, "3000");
  assert.equal(next.processManager, "pm2");
  assert.ok(next.cleanup.includes(".next/cache"));
  assert.equal(next.autoDetect, false);

  const go = applyFramework(next, "go");
  assert.equal(go.language, "go");
  assert.equal(go.install, "go build -o app .");
  assert.equal(go.run, "./app");
  assert.equal(go.processManager, "systemd");
}

{
  const apps = blocksToDeployApps(
    [{
      id: "1",
      name: "web",
      root: "apps/web",
      language: "node",
      install: "npm ci",
      run: "npm start",
      cleanup: "rm -rf .git",
      preStart: "npx prisma migrate deploy",
      port: "3000",
      processManager: "pm2",
      processName: "shop-web",
      processNameAuto: false,
      ecosystemFile: "",
      autoDetect: true,
      healthPath: "",
      healthPort: "",
      healthTimeout: "",
      startGroup: 0,
    }],
    { web: [{ key: "PORT", value: "3000", sensitive: false }] },
  );
  assert.equal(apps[0].process_name, "shop-web");
  assert.equal(apps.length, 1);
  assert.equal(apps[0].name, "web");
  assert.equal(apps[0].root, "apps/web");
  assert.equal(apps[0].port, 3000);
  assert.equal(apps[0].run, "npm start");
  assert.equal(apps[0].pre_start, "npx prisma migrate deploy");
  assert.equal(apps[0].process_manager, "pm2");
  assert.deepEqual(apps[0].env, [{ key: "PORT", value: "3000", sensitive: false }]);
}

console.log("project-blocks.test.ts: ok");
