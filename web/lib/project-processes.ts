import type { ConnectorResource, DeployApp, DeployInventory, DeployProject } from "@/lib/types";
import { qualifyProcessName } from "@/lib/process-name";

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
      objects.find(
        (o) =>
          o.kind === pm &&
          (o.resource.name === processName ||
            o.resource.ref === processName ||
            o.resource.name === app.name),
      ) ?? null;
    return {
      label: app.name || processName,
      processName,
      pm,
      resource: hit?.resource ?? null,
      connectorId: hit?.connectorId ?? null,
    };
  });
}

export function isProcessPaused(state?: string | null): boolean {
  const s = (state || "").toLowerCase().trim();
  return s === "paused" || s === "frozen";
}
