import assert from "node:assert/strict";
import {
  FRAMEWORKS,
  fieldsForFramework,
  frameworkPreset,
  runtimeLanguage,
} from "./frameworks.ts";

assert.ok(FRAMEWORKS.length >= 20, "curated framework list");
assert.ok(frameworkPreset("nextjs"));
assert.ok(frameworkPreset("nestjs"));
assert.ok(frameworkPreset("react"));
assert.ok(frameworkPreset("go"));
assert.ok(frameworkPreset("csharp"));
assert.ok(frameworkPreset("dotnet"));

assert.equal(runtimeLanguage("nextjs"), "node");
assert.equal(runtimeLanguage("nestjs"), "node");
assert.equal(runtimeLanguage("csharp"), "dotnet");
assert.equal(runtimeLanguage("fastapi"), "python");
assert.equal(runtimeLanguage("go"), "go");

const next = fieldsForFramework("nextjs")!;
assert.equal(next.language, "nextjs");
assert.match(next.install || "", /npm ci/);
assert.equal(next.run, "npm start");
assert.equal(next.port, "3000");
assert.equal(next.processManager, "pm2");

const nest = fieldsForFramework("nestjs")!;
assert.match(nest.run || "", /dist\/main/);

const cs = fieldsForFramework("csharp")!;
assert.equal(cs.processManager, "systemd");
assert.match(cs.install || "", /dotnet publish/);

assert.equal(fieldsForFramework("not-a-real-framework"), null);

console.log("frameworks.test.ts: ok");
