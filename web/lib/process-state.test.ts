import assert from "node:assert/strict";
import { processStateLabel, processStateTone } from "./process-state.ts";

assert.equal(processStateTone("online"), "ok");
assert.equal(processStateTone("running"), "ok");
assert.equal(processStateTone("stopped"), "neutral");
assert.equal(processStateTone("offline"), "neutral");
assert.equal(processStateTone("errored"), "danger");
assert.equal(processStateTone("launching"), "info");
assert.equal(processStateLabel(""), "unknown");
assert.equal(processStateLabel("online"), "online");

console.log("process-state: ok");
