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
  port: "",
  processManager: "none",
  autoDetect: true,
  healthPath: "",
  healthPort: "",
  healthTimeout: "",
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
      port: "",
      processManager: "none",
      autoDetect: true,
      healthPath: "",
      healthPort: "",
      healthTimeout: "",
    },
    "nextjs",
  );
  assert.equal(next.language, "nextjs");
  assert.equal(next.install, "npm ci && npm run build");
  assert.equal(next.run, "npm start");
  assert.equal(next.port, "3000");
  assert.equal(next.processManager, "pm2");
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
      port: "3000",
      processManager: "pm2",
      autoDetect: true,
      healthPath: "",
      healthPort: "",
      healthTimeout: "",
    }],
    { web: [{ key: "PORT", value: "3000", sensitive: false }] },
  );
  assert.equal(apps.length, 1);
  assert.equal(apps[0].name, "web");
  assert.equal(apps[0].root, "apps/web");
  assert.equal(apps[0].port, 3000);
  assert.equal(apps[0].run, "npm start");
  assert.equal(apps[0].process_manager, "pm2");
  assert.deepEqual(apps[0].env, [{ key: "PORT", value: "3000", sensitive: false }]);
}

console.log("project-blocks.test.ts: ok");
