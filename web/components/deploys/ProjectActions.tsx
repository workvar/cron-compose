"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import type { DeployProject, ListResponse, Server } from "@/lib/types";
import type { SelectOption } from "@/lib/ui-helpers";
import { listBranches } from "@/lib/git-detect";
import { SearchableSelect } from "@/components/SearchableSelect";
import { UserSwitcher } from "@/components/terminal/UserSwitcher";
import { HealthCheckFields, type HealthCheckValues } from "./HealthCheckFields";
import { RedeployButton } from "./RedeployButton";

const GIT_PROVIDERS = new Set(["github", "gitlab"]);

export function ProjectActions({ project }: { project: DeployProject }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState(project.name);
  const [serverId, setServerId] = useState(project.server_id);
  const [branch, setBranch] = useState(project.default_branch);
  const [install, setInstall] = useState(project.install_script);
  const [clonePath, setClonePath] = useState(project.clone_path);
  const [runAsUser, setRunAsUser] = useState(project.run_as_user || "");
  const [port, setPort] = useState(project.port ? String(project.port) : "");
  const [pm, setPm] = useState(project.process_manager);
  const [autoRollback, setAutoRollback] = useState(project.auto_rollback);
  const [needsRedeploy, setNeedsRedeploy] = useState(false);
  const [health, setHealth] = useState<HealthCheckValues>({
    path: project.health_path || "",
    port: project.health_port ? String(project.health_port) : "",
    timeout: project.health_timeout_seconds ? String(project.health_timeout_seconds) : "",
    deployTimeout: project.deploy_timeout_seconds ? String(project.deploy_timeout_seconds) : "",
  });

  const [servers, setServers] = useState<Server[] | null>(null);
  const [branches, setBranches] = useState<SelectOption[]>([]);
  const isGit = GIT_PROVIDERS.has(project.provider);

  useEffect(() => {
    if (!open) return;
    let live = true;
    setServers(null);
    fetch("/api/servers")
      .then((r) => r.json() as Promise<ListResponse<Server>>)
      .then((d) => {
        if (live) setServers(d.items || []);
      })
      .catch(() => {
        if (live) setServers([]);
      });
    return () => {
      live = false;
    };
  }, [open]);

  useEffect(() => {
    if (!open || !isGit) {
      setBranches([]);
      return;
    }
    let live = true;
    listBranches(project.provider, project.repo_full_name)
      .then((items) => {
        if (live) {
          setBranches(
            items.map((b) => ({
              value: b.name,
              label: b.default ? `${b.name} (default)` : b.name,
            })),
          );
        }
      })
      .catch(() => {
        if (live) setBranches([]);
      });
    return () => {
      live = false;
    };
  }, [open, isGit, project.provider, project.repo_full_name]);

  const serverOptions = useMemo(
    () => (servers || []).map((s) => ({ value: s.id, label: `${s.name} · ${s.status}` })),
    [servers],
  );

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
          server_id: serverId,
          default_branch: branch,
          install_script: install,
          clone_path: clonePath,
          run_as_user: runAsUser,
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
      setNeedsRedeploy(true);
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
              <label htmlFor="server">Server</label>
              {servers === null ? (
                <p className="field-hint">Loading servers…</p>
              ) : servers.length === 0 ? (
                <p className="field-hint">
                  No servers yet. <Link href="/servers/new">Add a server</Link>, then come back.
                </p>
              ) : (
                <SearchableSelect
                  id="server"
                  value={serverId}
                  onChange={setServerId}
                  options={serverOptions}
                  placeholder="Select a server…"
                  aria-label="Target server"
                />
              )}
              <p className="field-hint">Machine the agent runs this deploy on.</p>
            </div>
          </div>
          <div className="field">
            <label htmlFor="branch">Branch</label>
            {isGit ? (
              <SearchableSelect
                id="branch"
                value={branch}
                options={branches}
                allowCustom
                placeholder="main"
                onChange={setBranch}
                aria-label="Branch"
              />
            ) : (
              <input id="branch" value={branch} onChange={(e) => setBranch(e.target.value)} />
            )}
            <p className="field-hint">Pushes to this branch redeploy automatically.</p>
          </div>
          <div className="field">
            <label htmlFor="install">Install script</label>
            <textarea id="install" rows={3} value={install} onChange={(e) => setInstall(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="run-as">Deploy as</label>
            <UserSwitcher id="run-as" serverId={serverId} value={runAsUser} onChange={setRunAsUser} />
            <p className="field-hint">OS account for clone/install/start. Non-root uses ~/opt and ~/tmp.</p>
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
          <div className="cluster" style={{ marginTop: 8 }}>
            <button type="submit" className="button" disabled={busy || !serverId}>
              {busy ? "Saving…" : "Save"}
            </button>
            <button type="button" className="button secondary" onClick={() => setOpen(false)}>
              Close
            </button>
          </div>
          {needsRedeploy && (
            <div className="env-redeploy-bar" style={{ marginTop: 16, position: "relative" }}>
              <div>
                <div style={{ fontWeight: 700, color: "var(--text)" }}>Redeploy to apply settings</div>
                <p className="subtle" style={{ margin: "4px 0 0", fontSize: 13 }}>
                  Saved. Changing env or other deploy settings requires a redeploy so the agent applies
                  them on the server.
                </p>
              </div>
              <RedeployButton
                projectId={project.id}
                provider={project.provider}
                repo={project.repo_full_name}
                defaultBranch={branch || project.default_branch}
              />
            </div>
          )}
        </form>
      )}
    </>
  );
}
