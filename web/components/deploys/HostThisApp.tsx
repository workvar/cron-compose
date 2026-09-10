"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { DeployProject, DeployRun } from "@/lib/types";

export function HostThisApp({ project }: { project: DeployProject }) {
  const router = useRouter();
  const [pm, setPm] = useState(project.process_manager === "none" ? "pm2" : project.process_manager);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (project.process_manager && project.process_manager !== "none") {
    return null;
  }

  async function host() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${project.id}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ process_manager: pm }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = (await res.json()) as { project: DeployProject; run?: DeployRun };
      if (data.run?.id) {
        router.push(`/deploys/runs/${data.run.id}`);
        return;
      }
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="panel" style={{ marginTop: 18 }}>
      <div className="card-title">Host this app</div>
      <p className="subtle">No process manager is attached. Pick one to start it on the agent after clone/install.</p>
      <div className="cluster" style={{ marginTop: 12 }}>
        <select value={pm} onChange={(e) => setPm(e.target.value)}>
          <option value="pm2">PM2</option>
          <option value="systemd">systemd (user unit)</option>
          <option value="docker">Docker Compose</option>
        </select>
        <button type="button" className="button" disabled={busy} onClick={host}>
          {busy ? "Starting…" : "Attach and deploy"}
        </button>
        <a className="button secondary sm" href={`/app/connectors`}>Open Connectors</a>
      </div>
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}
