import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Connector, ListResponse, Server } from "@/lib/types";
import { IconPlus } from "@/components/icons";
import { DeployServerCards } from "@/components/deploys/DeployServerCards";

export default async function DeploysPage() {
  let servers: Server[] = [];
  let connectors: Connector[] = [];
  let error: string | null = null;
  try {
    const [serversData, connectorsData] = await Promise.all([
      apiGet<ListResponse<Server>>("/servers"),
      apiGet<ListResponse<Connector>>("/connectors").catch(() => ({ items: [] as Connector[] })),
    ]);
    servers = serversData.items;
    connectors = connectorsData.items;
  } catch (e) {
    error = (e as Error).message;
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Deploy</h1>
          <p className="subtle">Pick a server to view running processes, or import a new repository.</p>
        </div>
        <div className="page-head-actions">
          <Link href="/docs" className="button secondary">croncompose.yml docs</Link>
          <Link href="/deploys/new" className="button"><IconPlus /> New project</Link>
        </div>
      </div>

      {error && (
        <div className="form-error">Could not load servers: <code>{error}</code></div>
      )}

      {!error && <DeployServerCards servers={servers} connectors={connectors} />}
    </>
  );
}
