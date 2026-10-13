import assert from "node:assert/strict";
import {
  buildProcessRows,
  isProcessRunning,
  processActionRef,
} from "./project-processes.ts";
import type { DeployInventory, DeployProject } from "./types.ts";

const project = {
  name: "Peepal RP",
  process_manager: "pm2",
  apps: [
    { name: "Proxy", root: "proxy", process_manager: "pm2" },
    { name: "Backend", root: "backend", process_manager: "pm2" },
  ],
} as DeployProject;

const inventory = {
  server_id: "srv",
  total: 1,
  items: [
    {
      kind: "pm2",
      connectors: [{ id: "conn-pm2" } as DeployInventory["items"][number]["connectors"][number]],
      objects: [
        {
          id: "obj-1",
          connector_id: "conn-pm2",
          type: "object",
          ref: "4",
          name: "peepal-rp-proxy",
          state: "online",
          updated_at: "",
        },
      ],
      count: 1,
    },
  ],
} as DeployInventory;

const rows = buildProcessRows(project, inventory);
assert.equal(rows[0].resource?.name, "peepal-rp-proxy");
assert.equal(rows[0].connectorId, "conn-pm2");
assert.equal(processActionRef(rows[0]), "4");
assert.equal(rows[1].resource, null);
assert.equal(rows[1].connectorId, "conn-pm2");
assert.equal(processActionRef(rows[1]), "peepal-rp-backend");
assert.equal(isProcessRunning("online"), true);
assert.equal(isProcessRunning("stopped"), false);
assert.equal(isProcessRunning(""), false);

const systemd = buildProcessRows(
  {
    name: "shop",
    apps: [{ name: "api", root: ".", process_manager: "systemd", process_name: "shop-api" }],
  } as DeployProject,
  {
    server_id: "srv",
    total: 1,
    items: [
      {
        kind: "systemd",
        connectors: [{ id: "conn-sys" } as DeployInventory["items"][number]["connectors"][number]],
        objects: [
          {
            id: "obj-2",
            connector_id: "conn-sys",
            type: "object",
            ref: "shop-api.service",
            name: "shop-api",
            state: "running",
            updated_at: "",
          },
        ],
        count: 1,
      },
    ],
  } as DeployInventory,
);
assert.equal(systemd[0].resource?.ref, "shop-api.service");
assert.equal(processActionRef(systemd[0]), "shop-api.service");

const fromConnectors = buildProcessRows(
  project,
  { server_id: "srv", total: 0, items: [] } as DeployInventory,
  [{ id: "conn-fallback", kind: "pm2" } as import("./types.ts").Connector],
);
assert.equal(fromConnectors[0].connectorId, "conn-fallback");
assert.equal(fromConnectors[1].connectorId, "conn-fallback");
assert.equal(processActionRef(fromConnectors[1]), "peepal-rp-backend");

const systemdNameOnly = buildProcessRows(
  {
    name: "shop",
    apps: [{ name: "api", root: ".", process_manager: "systemd", process_name: "shop-api" }],
  } as DeployProject,
  null,
  [{ id: "conn-sys-2", kind: "systemd" } as import("./types.ts").Connector],
);
assert.equal(processActionRef(systemdNameOnly[0]), "shop-api.service");

{
  // ecosystem.config.js name `peepal-proxy` still matches app Proxy.
  const eco = buildProcessRows(project, {
    server_id: "srv",
    total: 1,
    items: [
      {
        kind: "pm2",
        connectors: [{ id: "conn-pm2" } as DeployInventory["items"][number]["connectors"][number]],
        objects: [
          {
            id: "obj-eco",
            connector_id: "conn-pm2",
            type: "object",
            ref: "7",
            name: "peepal-proxy",
            state: "online",
            attributes: { cwd: "/opt/apps/node/peepal-rp/current/proxy" },
            updated_at: "",
          },
        ],
        count: 1,
      },
    ],
  } as DeployInventory);
  assert.equal(eco[0].resource?.name, "peepal-proxy");
  assert.equal(processActionRef(eco[0]), "7");
  assert.equal(eco[1].resource, null);
}

console.log("project-processes: ok");
