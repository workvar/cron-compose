import type { Connector, ConnectorResource, DeployApp, DeployInventory, DeployProject } from "./types";
import { qualifyProcessName } from "./process-name";

/** One project app matched against live deploy-inventory. */
export type ProcessRow = {
  label: string;
  processName: string;
  pm: string;
  resource: ConnectorResource | null;
  connectorId: string | null;
};

/** Map project apps onto inventory objects for day-2 process controls. */
export function buildProcessRows(
  project: DeployProject,
  inventory: DeployInventory | null,
  connectors: Connector[] = [],
): ProcessRow[] {
  const apps: DeployApp[] =
    project.apps?.length > 0
      ? project.apps
      : [
          {
            name: project.name,
            root: project.root_directory || ".",
            process_manager: project.process_manager,
            process_name: qualifyProcessName(project.name, project.name),
          },
        ];

  const objects: { kind: string; connectorId: string; resource: ConnectorResource }[] = [];
  for (const bucket of inventory?.items ?? []) {
    const fallbackConn = bucket.connectors[0]?.id ?? "";
    for (const o of bucket.objects) {
      const extra = o as ConnectorResource & { connector_id?: string };
      objects.push({
        kind: bucket.kind,
        connectorId: extra.connector_id || o.attributes?.connector_id || fallbackConn,
        resource: o,
      });
    }
  }

  return apps.map((app) => {
    const pm = (app.process_manager || project.process_manager || "none").trim() || "none";
    const processName =
      app.process_name || qualifyProcessName(project.name, app.name || app.root || "app");
    const hit =
      objects.find((o) => o.kind === pm && sameProcess(o.resource, processName, app.name || "")) ??
      null;
    const bucket = (inventory?.items ?? []).find((b) => b.kind === pm);
    const fromList = connectors.find((c) => c.kind === pm)?.id ?? null;
    return {
      label: app.name || processName,
      processName,
      pm,
      resource: hit?.resource ?? null,
      // A failed deploy often dies before the start step, so the process is missing
      // from inventory. Prefer the matched object, then the inventory bucket, then
      // any connector of that kind on the server.
      connectorId: hit?.connectorId || bucket?.connectors[0]?.id || fromList || null,
    };
  });
}

function normName(s: string): string {
  return s.trim().toLowerCase().replace(/\.service$/, "");
}

function sameProcess(resource: ConnectorResource, processName: string, appName: string): boolean {
  const got = [normName(resource.name), normName(resource.ref)].filter((s) => s.length > 0);
  const appSlug = normName(appName).replace(/[^a-z0-9._-]+/g, "-").replace(/^-+|-+$/g, "");
  const want = [normName(processName), normName(appName), appSlug].filter((s) => s.length >= 2);
  for (const g of got) {
    // Numeric pm2 ids are refs, not names.
    if (/^\d+$/.test(g)) continue;
    if (want.includes(g)) return true;
    // ecosystem.config.js often names the process `peepal-proxy` while we register
    // `peepal-rp-proxy`. Treat a trailing `-<app>` as the same process.
    if (appSlug.length >= 2 && (g === appSlug || g.endsWith(`-${appSlug}`))) return true;
  }
  // cwd from pm2 jlist: …/current/proxy matches app root "proxy".
  const cwd = normName(resource.attributes?.cwd || "");
  if (appSlug.length >= 2 && cwd) {
    if (cwd.endsWith(`/${appSlug}`) || cwd.endsWith(`\\${appSlug}`) || cwd.endsWith(appSlug)) {
      return true;
    }
  }
  return false;
}

/** Ref passed to connector actions. Falls back to the qualified process name. */
export function processActionRef(row: ProcessRow): string {
  const ref = (row.resource?.ref || "").trim();
  if (ref) return ref;
  if (row.pm === "systemd" && row.processName && !row.processName.endsWith(".service")) {
    return `${row.processName}.service`;
  }
  return row.processName;
}

export function isProcessPaused(state?: string | null): boolean {
  const s = (state || "").toLowerCase().trim();
  return s === "paused" || s === "frozen";
}

export function isProcessRunning(state?: string | null): boolean {
  const s = (state || "").toLowerCase().trim();
  return s === "online" || s === "running" || s === "active" || s === "listening";
}
