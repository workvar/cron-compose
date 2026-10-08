import assert from "node:assert/strict";
import { buildDeployLive, type ProgressApp } from "./deploy-progress.ts";

const web: ProgressApp = {
  name: "college-connect-web",
  root: ".",
  language: "node",
  install: "npm install && npm run build",
  processManager: "pm2",
};

function lines(text: string) {
  return [{ seq: 1, chunk: text.endsWith("\n") ? text : text + "\n" }];
}

{
  const view = buildDeployLive({
    status: "failed",
    apps: [web],
    logs: lines(`install: npm install && npm run build (in /opt/apps/web/releases/abc)
added 32 packages, and audited 33 packages in 21s

> college-connect-web@0.0.0 build
> next build

▲ Next.js 15.5.25
Creating an optimized production build ...
✓ Compiled successfully in 43s
clone: removing tmp clone /home/pi/tmp/croncompose-clone-01M4CTEM
FAILED — install: npm install && npm run build: context canceled
`),
  });

  const proc = view.processes[0];
  const installing = proc.steps.find((s) => s.label === "Installing college-connect-web");
  const building = proc.steps.find((s) => s.label === "Building college-connect-web");
  const starting = proc.steps.find((s) => s.kind === "start");
  assert.equal(installing?.state, "done");
  assert.equal(building?.state, "failed");
  assert.equal(starting?.state, "skipped");
  assert.equal(view.shared.find((s) => s.kind === "clone")?.state, "done");
  assert.equal(view.shared.find((s) => s.kind === "release")?.state, "skipped");
  assert.ok(view.percent < 100, `percent ${view.percent} should stay under 100`);
  assert.equal(view.headline, "Building college-connect-web");
  assert.ok(view.lines.some((l) => l.text.includes("Compiled successfully")));
  assert.ok(view.lines.some((l) => l.pane === proc.id && l.text.startsWith("FAILED")));
  const cloneLine = view.lines.find((l) => l.text.includes("removing tmp clone"));
  assert.equal(cloneLine?.pane, "shared");
}

{
  const view = buildDeployLive({
    status: "running",
    apps: [web],
    logs: lines(`preflight: git: ok
clone: cloning https://example.com/repo (main) into tmp
clone: tmp clone ready (abc1234)
install: npm install && npm run build (in /opt/apps/web/releases/abc)
added 32 packages
> college-connect-web@0.0.0 build
> next build
`),
  });
  const proc = view.processes[0];
  assert.equal(proc.steps.find((s) => s.kind === "install")?.state, "done");
  assert.equal(proc.steps.find((s) => s.kind === "build")?.state, "active");
  assert.ok((proc.steps.find((s) => s.kind === "build")?.percent || 0) > 0);
  assert.ok(view.percent > 0 && view.percent < 100);
  assert.equal(view.headline, "Building college-connect-web");
}

{
  const apps: ProgressApp[] = [
    { name: "web", root: "apps/web", language: "node", install: "npm ci && npm run build", processManager: "pm2" },
    {
      name: "api",
      root: "apps/api",
      language: "go",
      install: "go mod download && go build -o api .",
      processManager: "systemd",
      healthPath: "/healthz",
    },
  ];
  const view = buildDeployLive({
    status: "running",
    apps,
    logs: lines(`preflight: git: ok
clone: tmp clone ready (abc1234)
install: npm ci && npm run build (in /opt/releases/r1/apps/web)
added 10 packages
> web@1.0.0 build
> next build
✓ Compiled successfully in 12s
install: go mod download && go build -o api . (in /opt/releases/r1/apps/api)
go: downloading example.com/mod v1.2.3
`),
  });
  assert.equal(view.processes.length, 2);
  const webProc = view.processes[0];
  const apiProc = view.processes[1];
  assert.equal(webProc.steps.find((s) => s.kind === "build")?.state, "done");
  assert.equal(apiProc.steps.find((s) => s.kind === "install")?.state, "active");
  assert.ok(view.lines.some((l) => l.pane === webProc.id && l.text.includes("next build")));
  assert.ok(view.lines.some((l) => l.pane === apiProc.id && l.text.includes("downloading")));
  assert.ok(view.lines.some((l) => l.pane === "shared" && l.text.includes("tmp clone ready")));
  assert.equal(view.headline, "Installing api");
}

{
  const view = buildDeployLive({
    status: "succeeded",
    apps: [web],
    logs: lines(`preflight: git: ok
clone: tmp clone ready (abc1234)
install: npm install && npm run build (in /opt/releases/abc)
> college-connect-web@0.0.0 build
> next build
release: current -> abc
pm2 start npm --name college-connect-web -- start
`),
  });
  assert.equal(view.percent, 100);
  assert.equal(view.headline, "Finished");
  assert.ok(view.shared.every((s) => s.state === "done"));
  assert.ok(view.processes[0].steps.every((s) => s.state === "done"));
}

console.log("deploy-progress: ok");
