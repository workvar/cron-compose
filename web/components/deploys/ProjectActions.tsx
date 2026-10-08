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
    setName(project.name);
    setServerId(project.server_id);
    setBranch(project.default_branch);
    setInstall(project.install_script);
    setClonePath(project.clone_path);
    setRunAsUser(project.run_as_user || "");
    setPort(project.port ? String(project.port) : "");
    setPm(project.process_manager);
    setAutoRollback(project.auto_rollback);
    setHealth({
      path: project.health_path || "",
      port: project.health_port ? String(project.health_port) : "",
      timeout: project.health_timeout_seconds ? String(project.health_timeout_seconds) : "",
      deployTimeout: project.deploy_timeout_seconds ? String(project.deploy_timeout_seconds) : "",
    });
    setError(null);
  }, [open, project]);

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

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

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
      <button type="button" className="button secondary" onClick={() => setOpen(true)}>
        Edit
      </button>
      <button type="button" className="button secondary" onClick={remove} disabled={busy}>
        Delete
      </button>

      {open && (
        <div className="modal-backdrop" role="presentation" onClick={() => setOpen(false)}>
          <form
            className="modal-panel deploy-edit-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="deploy-edit-title"
            onClick={(e) => e.stopPropagation()}
            onSubmit={save}
          >
            <div className="row" style={{ alignItems: "flex-start", marginBottom: 4 }}>
              <div>
                <h2 id="deploy-edit-title" style={{ margin: 0 }}>Edit deploy</h2>
                <p className="subtle" style={{ margin: "6px 0 0", fontSize: 13 }}>
                  {project.provider}/{project.repo_full_name}
                </p>
              </div>
              <button type="button" className="button ghost sm" onClick={() => setOpen(false)} aria-label="Close">
                Close
              </button>
            </div>

            <div className="deploy-edit-section">
              <div className="deploy-edit-section-title">Target</div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="deploy-edit-name">Name</label>
                  <input id="deploy-edit-name" value={name} onChange={(e) => setName(e.target.value)} />
                </div>
                <div className="field">
                  <label htmlFor="deploy-edit-server">Server</label>
                  {servers === null ? (
                    <p className="field-hint">Loading servers…</p>
                  ) : servers.length === 0 ? (
                    <p className="field-hint">
                      No servers yet. <Link href="/servers/new">Add a server</Link>, then come back.
                    </p>
                  ) : (
                    <SearchableSelect
                      id="deploy-edit-server"
                      value={serverId}
                      onChange={setServerId}
                      options={serverOptions}
                      placeholder="Select a server…"
                      aria-label="Target server"
                    />
                  )}
                </div>
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="deploy-edit-branch">Branch</label>
                  {isGit ? (
                    <SearchableSelect
                      id="deploy-edit-branch"
                      value={branch}
                      options={branches}
                      allowCustom
                      placeholder="main"
                      onChange={setBranch}
                      aria-label="Branch"
                    />
                  ) : (
                    <input id="deploy-edit-branch" value={branch} onChange={(e) => setBranch(e.target.value)} />
                  )}
                  <p className="field-hint">Pushes to this branch redeploy automatically.</p>
                </div>
                <div className="field">
                  <label htmlFor="deploy-edit-run-as">Deploy as</label>
                  <UserSwitcher id="deploy-edit-run-as" serverId={serverId} value={runAsUser} onChange={setRunAsUser} />
                  <p className="field-hint">OS account for clone/install/start.</p>
                </div>
              </div>
              <div className="field">
                <label htmlFor="deploy-edit-clone">Clone path</label>
                <input id="deploy-edit-clone" value={clonePath} onChange={(e) => setClonePath(e.target.value)} spellCheck={false} />
              </div>
            </div>

            <div className="deploy-edit-section">
              <div className="deploy-edit-section-title">Build &amp; process</div>
              <div className="field">
                <label htmlFor="deploy-edit-install">Install script</label>
                <textarea
                  id="deploy-edit-install"
                  rows={3}
                  value={install}
                  onChange={(e) => setInstall(e.target.value)}
                  spellCheck={false}
                />
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="deploy-edit-port">PORT</label>
                  <input id="deploy-edit-port" value={port} onChange={(e) => setPort(e.target.value)} placeholder="auto" />
                </div>
                <div className="field">
                  <label htmlFor="deploy-edit-pm">Process manager</label>
                  <select id="deploy-edit-pm" value={pm} onChange={(e) => setPm(e.target.value)}>
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
              </div>
            </div>

            <div className="deploy-edit-section">
              <div className="deploy-edit-section-title">Health</div>
              <HealthCheckFields value={health} onChange={setHealth} appPort={project.port} idPrefix="deploy-edit-" />
            </div>

            {error && <p className="form-error">{error}</p>}

            <div className="cluster" style={{ marginTop: 4, justifyContent: "flex-end" }}>
              <button type="button" className="button secondary" onClick={() => setOpen(false)}>
                Cancel
              </button>
              <button type="submit" className="button" disabled={busy || !serverId}>
                {busy ? "Saving…" : "Save changes"}
              </button>
            </div>

            {needsRedeploy && (
              <div className="env-redeploy-bar" style={{ marginTop: 16 }}>
                <div>
                  <div style={{ fontWeight: 700, color: "var(--text)" }}>Redeploy to apply</div>
                  <p className="subtle" style={{ margin: "4px 0 0", fontSize: 13 }}>
                    Settings are saved. Redeploy so the agent applies them on the server.
                  </p>
                </div>
                <RedeployButton
                  projectId={project.id}
                  provider={project.provider}
                  repo={project.repo_full_name}
                  defaultBranch={branch || project.default_branch}
                  compact
                />
              </div>
            )}
          </form>
        </div>
      )}
    </>
  );
}
