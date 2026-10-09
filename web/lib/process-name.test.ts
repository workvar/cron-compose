import assert from "node:assert/strict";
import { qualifyProcessName } from "./process-name.ts";

assert.equal(qualifyProcessName("shop", "web"), "shop-web");
assert.equal(qualifyProcessName("Shop App", "Web API"), "shop-app-web-api");
assert.equal(qualifyProcessName("shop", "shop"), "shop");
assert.equal(qualifyProcessName("shop", ""), "shop");
assert.equal(qualifyProcessName("", "web"), "web");
assert.equal(qualifyProcessName("", ""), "app");
assert.equal(qualifyProcessName("shop", "shop-web"), "shop-web");
assert.equal(qualifyProcessName("College Connect", "api"), "college-connect-api");

console.log("process-name: ok");
