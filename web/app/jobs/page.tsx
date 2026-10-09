import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { Job, ListResponse, Server } from "@/lib/types";
import { IconPlus } from "@/components/icons";
import { JobServerCards } from "@/components/jobs/JobServerCards";

export default async function JobsPage() {
  let jobs: Job[] = [];
  let servers: Server[] = [];
  let error: string | null = null;
  try {
    const [jobsData, serversData] = await Promise.all([
      apiGet<ListResponse<Job>>("/jobs"),
      apiGet<ListResponse<Server>>("/servers"),
    ]);
    jobs = jobsData.items;
    servers = serversData.items;
  } catch (e) {
    error = (e as Error).message;
  }

  const newJobHref = servers.length > 0 ? `/servers/${servers[0].id}/jobs/new` : "/servers/new";

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Jobs</h1>
          <p className="subtle">Pick a server to view scheduled jobs, or create a new one.</p>
        </div>
        <div className="page-head-actions">
          <Link href={newJobHref} className="button"><IconPlus /> New job</Link>
        </div>
      </div>

      {error && (
        <div className="form-error">Could not load jobs: <code>{error}</code></div>
      )}

      {!error && <JobServerCards servers={servers} jobs={jobs} />}
    </>
  );
}
