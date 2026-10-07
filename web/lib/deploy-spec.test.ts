import assert from "node:assert/strict";
import {
  draftToYaml,
  emptyAdvanced,
  matchServer,
  parseRepoUrl,
  specToDraft,
  yamlScalar,
} from "./deploy-spec.ts";

const servers = [
  { id: "srv_1", name: "Pi-Home" },
  { id: "srv_2", name: "ec2" },
];

assert.equal(matchServer(servers, "srv_2"), "srv_2");
assert.equal(matchServer(servers, "pi-home"), "srv_1");
assert.equal(matchServer(servers, "nope"), "");
assert.equal(matchServer(servers, undefined), "");

// Single-app spec: top-level fields become one block.
{
  const d = specToDraft(
    {
      provider: "github",
      repo: "acme/shop",
      branch: "prod",
      server: "ec2",
      install: "npm ci && npm run build",
      language: "node",
      port: 3000,
      process_manager: "pm2",
      env: { NODE_ENV: "production" },
      health: { path: "/healthz", timeout: 30 },
      auto_rollback: true,
    },
    servers,
  );
  assert.equal(d.serverId, "srv_2");
  assert.equal(d.branch, "prod");
  assert.equal(d.blocks.length, 1);
  assert.equal(d.blocks[0].name, "shop");
  assert.equal(d.blocks[0].root, ".");
  assert.equal(d.blocks[0].port, "3000");
  assert.equal(d.blocks[0].processManager, "pm2");
  assert.deepEqual(d.globalEnv, [{ key: "NODE_ENV", value: "production", sensitive: false }]);
  assert.deepEqual(d.appEnv.shop, []);
  assert.equal(d.advanced.healthPath, "/healthz");
  assert.equal(d.advanced.healthTimeout, "30");
  assert.equal(d.advanced.autoRollback, true);
}

// Multi-app: top-level env stays shared; per-app list is only that app's own vars.
{
  const d = specToDraft(
    {
      provider: "github",
      repo: "acme/mono",
      process_manager: "systemd",
      env: { LOG: "info", SHARED: "1" },
      apps: [
        { name: "web", root: "apps/web", install: "pnpm i", env: [{ key: "LOG", value: "debug", sensitive: false }] },
        { name: "", root: "apps/api", process_manager: "pm2" },
      ],
    },
    servers,
  );
  assert.equal(d.serverId, undefined);
  assert.deepEqual(d.blocks.map((b) => [b.name, b.processManager]), [["web", "systemd"], ["api", "pm2"]]);
  assert.deepEqual(d.globalEnv.map((v) => `${v.key}=${v.value}`), ["LOG=info", "SHARED=1"]);
  assert.deepEqual(d.appEnv.web.map((v) => `${v.key}=${v.value}`), ["LOG=debug"]);
  assert.deepEqual(d.appEnv.api, []);
}

assert.deepEqual(parseRepoUrl("acme/web"), { provider: "github", fullName: "acme/web" });
assert.deepEqual(parseRepoUrl("https://github.com/acme/web.git"), { provider: "github", fullName: "acme/web" });
assert.deepEqual(parseRepoUrl("github.com/acme/web/tree/main"), { provider: "github", fullName: "acme/web" });
assert.deepEqual(parseRepoUrl("git@gitlab.com:grp/sub/proj.git"), { provider: "gitlab", fullName: "grp/sub/proj" });
assert.deepEqual(parseRepoUrl("https://gitlab.com/grp/proj/-/tree/main"), { provider: "gitlab", fullName: "grp/proj" });
assert.equal(parseRepoUrl("https://bitbucket.org/a/b"), null);
assert.equal(parseRepoUrl("hello"), null);
assert.equal(parseRepoUrl(""), null);

assert.equal(yamlScalar("production"), "production");
assert.equal(yamlScalar("/opt/apps/node/web"), "/opt/apps/node/web");
assert.equal(yamlScalar("https://x.example.com"), "https://x.example.com");
assert.equal(yamlScalar("true"), '"true"');
assert.equal(yamlScalar("3000"), '"3000"');
assert.equal(yamlScalar("npm ci && npm run build"), '"npm ci && npm run build"');
assert.equal(yamlScalar("- x"), '"- x"');
assert.equal(yamlScalar("@scope/pkg"), '"@scope/pkg"');
assert.equal(yamlScalar("a: b"), '"a: b"');
assert.equal(yamlScalar(""), '""');

{
  const yml = draftToYaml({
    name: "shop",
    provider: "github",
    repo: "acme/shop",
    branch: "main",
    server: "ec2",
    blocks: [{
      id: "b1",
      name: "shop",
      root: ".",
      language: "node",
      install: "npm ci",
      run: "npm start",
      port: "3000",
      processManager: "pm2",
      autoDetect: false,
      healthPath: "",
      healthPort: "",
      healthTimeout: "",
    }],
    globalEnv: [{ key: "NODE_ENV", value: "production", sensitive: false }],
    appEnv: {
      shop: [
        { key: "PORT_HINT", value: "3000", sensitive: false },
        { key: "DB_PASSWORD", value: "", sensitive: true },
      ],
    },
    advanced: { ...emptyAdvanced, healthPath: "/healthz", autoRollback: true },
  });
  assert.match(yml, /^# croncompose\.yml/);
  assert.match(yml, /\nversion: 1\n/);
  assert.match(yml, /\nhealth:\n  path: \/healthz\n/);
  assert.match(yml, /\nauto_rollback: true\n/);
  assert.match(yml, /\nenv:\n  NODE_ENV: production\n/);
  assert.match(yml, /\n    install: npm ci\n/);
  assert.match(yml, /\n    port: 3000\n/);
  assert.match(yml, /\n      PORT_HINT: "3000"\n/);
  assert.doesNotMatch(yml, /DB_PASSWORD/);
  assert.match(yml, /1 sensitive variable left out/);
}

// File overrides detection; unset fields fall back to it. A single app inherits port.
{
  const d = specToDraft(
    { provider: "github", repo: "acme/one", port: 5006, apps: [{ name: "web", root: "." }] },
    servers,
    { language: "node", install: "npm install" },
  );
  assert.equal(d.blocks[0].language, "node");
  assert.equal(d.blocks[0].install, "npm install");
  assert.equal(d.blocks[0].port, "5006");
  const two = specToDraft(
    { provider: "github", repo: "acme/two", port: 5006, install: "make", apps: [{ name: "a", root: "a" }, { name: "b", root: "b" }] },
    servers,
    { language: "go", install: "go build" },
  );
  assert.deepEqual(two.blocks.map((b) => [b.port, b.install, b.language]), [["", "make", "go"], ["", "make", "go"]]);
}

console.log("deploy-spec.test.ts: ok");
