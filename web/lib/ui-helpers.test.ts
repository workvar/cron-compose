import assert from "node:assert/strict";
import { filterMappedPorts, filterSelectOptions, shouldShowServerPromo } from "./ui-helpers.ts";

{
  const opts = [
    { value: "global", label: "global" },
    { value: "server", label: "server" },
    { value: "job", label: "job" },
  ];
  assert.deepEqual(filterSelectOptions(opts, ""), opts);
  assert.deepEqual(filterSelectOptions(opts, "ser"), [{ value: "server", label: "server" }]);
  assert.deepEqual(filterSelectOptions(opts, "JOB"), [{ value: "job", label: "job" }]);
  assert.deepEqual(filterSelectOptions(opts, "xyz"), []);
}

assert.equal(shouldShowServerPromo(0), true);
assert.equal(shouldShowServerPromo(1), false);
assert.equal(shouldShowServerPromo(3), false);

{
  const rows = [
    {
      proto: "tcp", address: "0.0.0.0", port: 3107, pid: 12, process: "next-server",
      ref: "web", name: "web", protected: false, connector_id: "c1",
      server_id: "s1", server_name: "local-pi", kind: "pm2", label: "Next.js UI",
    },
    {
      proto: "tcp", address: "127.0.0.1", port: 8787, pid: 9, process: "control-plane",
      ref: "api", name: "api", protected: false, connector_id: "c1",
      server_id: "s1", server_name: "local-pi", kind: "pm2",
    },
  ];
  assert.equal(filterMappedPorts(rows, "").length, 2);
  assert.equal(filterMappedPorts(rows, "next").length, 1);
  assert.equal(filterMappedPorts(rows, "3107")[0].port, 3107);
  assert.equal(filterMappedPorts(rows, "control")[0].process, "control-plane");
  assert.equal(filterMappedPorts(rows, "xyz").length, 0);
}

console.log("ok");
