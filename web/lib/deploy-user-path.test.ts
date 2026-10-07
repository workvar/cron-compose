import assert from "node:assert/strict";
import { clonePathForUser, userTmpDir } from "./deploy-user-path.ts";

assert.equal(
  clonePathForUser("/opt/apps/node/college-connect", "pi", "/home/pi"),
  "/home/pi/opt/apps/node/college-connect",
);
assert.equal(
  clonePathForUser("/opt/apps/node/x", "root", "/root"),
  "/opt/apps/node/x",
);
assert.equal(
  clonePathForUser("/home/pi/opt/apps/node/x", "pi", "/home/pi"),
  "/home/pi/opt/apps/node/x",
);
assert.equal(userTmpDir("pi", "/home/pi"), "/home/pi/tmp");
assert.equal(userTmpDir("", "/home/pi"), "/tmp");
