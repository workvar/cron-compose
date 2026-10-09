import assert from "node:assert/strict";
import { deployDurationMs, formatDeployDuration, shortCommit } from "./deploy-duration.ts";

assert.equal(formatDeployDuration(450), "450ms");
assert.equal(formatDeployDuration(12_000), "12s");
assert.equal(formatDeployDuration(95_000), "1m 35s");
assert.equal(formatDeployDuration(3_600_000), "1h");
assert.equal(formatDeployDuration(3_900_000), "1h 5m");

assert.equal(shortCommit("abc1234deadbeef"), "abc1234");
assert.equal(shortCommit("abc"), "abc");
assert.equal(shortCommit(""), "");

const ms = deployDurationMs("2026-10-09T10:00:00.000Z", "2026-10-09T10:01:30.000Z");
assert.equal(ms, 90_000);

assert.equal(deployDurationMs(undefined, "2026-10-09T10:01:30.000Z"), null);

console.log("deploy-duration: ok");
