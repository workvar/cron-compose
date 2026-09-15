"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { DeployProject } from "@/lib/types";
import { HealthCheckFields, type HealthCheckValues } from "./HealthCheckFields";

export function ProjectActions({ project }: { project: DeployProject }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState(project.name);
  const [branch, setBranch] = useState(project.default_branch);
  const [install, setInstall] = useState(project.install_script);
  const [clonePath, setClonePath] = useState(project.clone_path);
  const [port, setPort] = useState(project.port ? String(project.port) : "");
  const [pm, setPm] = useState(project.process_manager);
  const [autoRollback, setAutoRollback] = useState(project.auto_rollback);
  const [health, setHealth] = useState<HealthCheckValues>({
    path: project.health_path || "",
    port: project.health_port ? String(project.health_port) : "",
    timeout: project.health_timeout_seconds ? String(project.health_timeout_seconds) : "",
    deployTimeout: project.deploy_timeout_seconds ? String(project.deploy_timeout_seconds) : "",
  });

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${project.id}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          name,
          default_branch: branch,
          install_script: install,
          clone_path: clonePath,
          port: port ? Number(port) : 0,
          process_manager: pm,
          auto_rollback: autoRollback,
          health_path: health.path.trim(),
          health_port: health.port ? Number(health.port) : 0,
          health_timeout_seconds: health.timeout ? Number(health.timeout) : 60,
          deploy_timeout_seconds: health.deployTimeout ? Number(health.deployTimeout) : 0,
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      setOpen(false);
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!confirm(`Delete deploy ${project.name}?`)) return;
    setBusy(true);
    try {
      const res = await fetch(`/api/deploys/${project.id}`, { method: "DELETE" });
      if (!res.ok && res.status !== 204) throw new Error(await res.text());
      router.push("/deploys");
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <>
      <button type="button" className="button secondary" onClick={() => setOpen((v) => !v)}>
        Edit
      </button>
      <button type="button" className="button secondary" onClick={remove} disabled={busy}>
        Delete
      </button>
      {open && (
        <form onSubmit={save} className="panel" style={{ marginTop: 16, maxWidth: 640 }}>
          <div className="grid-2">
            <div className="field">
              <label htmlFor="name">Name</label>
              <input id="name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="branch">Branch</label>
              <input id="branch" value={branch} onChange={(e) => setBranch(e.target.value)} />
            </div>
          </div>
          <div className="field">
            <label htmlFor="install">Install script</label>
            <textarea id="install" rows={3} value={install} onChange={(e) => setInstall(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="clone">Clone path</label>
            <input id="clone" value={clonePath} onChange={(e) => setClonePath(e.target.value)} />
          </div>
          <div className="grid-2">
            <div className="field">
              <label htmlFor="port">PORT</label>
              <input id="port" value={port} onChange={(e) => setPort(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="pm">Process manager</label>
              <select id="pm" value={pm} onChange={(e) => setPm(e.target.value)}>
                <option value="none">None</option>
                <option value="pm2">PM2</option>
                <option value="systemd">systemd</option>
                <option value="docker">Docker</option>
              </select>
            </div>
          </div>
          <div className="field">
            <label className="cluster" style={{ gap: 8, alignItems: "center" }}>
              <input type="checkbox" checked={autoRollback} onChange={(e) => setAutoRollback(e.target.checked)} />
              Roll back automatically on a failed deploy
            </label>
            <p className="field-hint">
              Redeploys the last successful commit and restarts the process manager on it. Off by default.
            </p>
          </div>
          <HealthCheckFields value={health} onChange={setHealth} appPort={project.port} />
          {error && <p className="form-error">{error}</p>}
          <button type="submit" className="button" disabled={busy}>{busy ? "Saving…" : "Save"}</button>
        </form>
      )}
    </>
  );
}
