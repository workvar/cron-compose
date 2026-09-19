import assert from "node:assert/strict";
import {
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
assert.equal(nameFromRoot(".", "acme/widgets"), "widgets");
assert.equal(nameFromRoot("apps/web", "acme/widgets"), "web");

const dup = ensureUniqueBlockNames([
  { id: "1", name: "web", root: "a", language: "", install: "", port: "", processManager: "none" },
  { id: "2", name: "web", root: "b", language: "", install: "", port: "", processManager: "none" },
]);
assert.equal(dup[1].name, "web-2");

assert.equal(
  hasDuplicateRoots([
    { id: "1", name: "a", root: "apps/web", language: "", install: "", port: "", processManager: "none" },
    { id: "2", name: "b", root: "apps/web", language: "", install: "", port: "", processManager: "none" },
  ]),
  true,
);

assert.equal(
  blocksReady([{ id: "1", name: "x", root: "", language: "", install: "", port: "", processManager: "none" }]),
  false,
);

assert.equal(
  blocksReady([{ id: "1", name: "x", root: "apps/web", language: "", install: "", port: "", processManager: "none" }]),
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
  assert.equal(block.processManager, "pm2");
}

{
  const apps = blocksToDeployApps(
    [{ id: "1", name: "web", root: "apps/web", language: "node", install: "npm ci", port: "3000", processManager: "pm2" }],
    { web: [{ key: "PORT", value: "3000", sensitive: false }] },
  );
  assert.equal(apps.length, 1);
  assert.equal(apps[0].name, "web");
  assert.equal(apps[0].root, "apps/web");
  assert.equal(apps[0].port, 3000);
  assert.equal(apps[0].process_manager, "pm2");
  assert.deepEqual(apps[0].env, [{ key: "PORT", value: "3000", sensitive: false }]);
}

console.log("project-blocks.test.ts: ok");
