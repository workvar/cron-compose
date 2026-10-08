import assert from "node:assert/strict";
import { stripAnsi } from "./strip-ansi";

assert.equal(stripAnsi("\u001b[1G\u001b[0K-\u001b[1G\u001b[0K|"), "-|");
assert.equal(stripAnsi("\u001b[32mok\u001b[0m"), "ok");
assert.equal(stripAnsi("a\rb\n"), "ab\n");
console.log("strip-ansi: ok");
