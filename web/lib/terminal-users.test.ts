import assert from "node:assert/strict";
import { visibleTerminalUsers } from "./terminal-users.ts";

const users = [
  { username: "a", uid: 1000, home: "", shell: "/bin/bash", available: true },
  { username: "b", uid: 1001, home: "", shell: "/bin/bash", available: false },
];

assert.equal(visibleTerminalUsers(users, false).length, 1);
assert.equal(visibleTerminalUsers(users, false)[0].username, "a");
assert.equal(visibleTerminalUsers(users, true).length, 2);

{
  const none = [
    { username: "root", uid: 0, home: "/root", shell: "/bin/bash", available: false },
  ];
  assert.equal(visibleTerminalUsers(none, false).length, 0);
  assert.equal(visibleTerminalUsers(none, true).length, 1);
  assert.deepEqual(visibleTerminalUsers([], false), []);
}

console.log("ok");
