"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Stepper, type StepDef } from "@/components/jobwizard/Stepper";
import { GitConnections } from "@/components/deploys/GitConnections";
import { IconChevronLeft, IconChevronRight, IconCheck } from "@/components/icons";
import type {
  DeployApp,
  DeployInspect,
  DeployProject,
  DeployRun,
  GitConnection,
  GitRepo,
  ListResponse,
  Server,
} from "@/lib/types";

const STEPS: StepDef[] = [
  { title: "Repository", desc: "Pick a git repo" },
  { title: "Build", desc: "Language and install" },
  { title: "Runtime", desc: "Server, env, process" },
  { title: "Review", desc: "Clone and install" },
];

type Draft = {
  provider: string;
  repo: GitRepo | null;
  inspect: DeployInspect | null;
  serverId: string;
  language: string;
  install: string;
  root: string;
  clonePath: string;
  branch: string;
  port: string;
  processManager: string;
  selectedApps: string[];
  envText: string;
};

const empty: Draft = {
  provider: "github",
  repo: null,
  inspect: null,
  serverId: "",
  language: "node",
  install: "",
  root: ".",
  clonePath: "",
  branch: "main",
  port: "",
  processManager: "none",
  selectedApps: [],
  envText: "",
};

export default function NewDeployPage() {
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<Draft>(empty);
  const [conns, setConns] = useState<GitConnection[]>([]);
  const [repos, setRepos] = useState<GitRepo[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<{ project: DeployProject; run: DeployRun; warnings?: string[] } | null>(null);

  useEffect(() => {
    fetch("/api/git/connections")
      .then((r) => r.json() as Promise<ListResponse<GitConnection>>)
      .then((d) => {
        setConns(d.items || []);
        const first = d.items?.[0]?.provider;
        if (first) setDraft((d0) => ({ ...d0, provider: first }));
      })
      .catch(() => setConns([]));
    fetch("/api/servers")
      .then((r) => r.json() as Promise<ListResponse<Server>>)
      .then((d) => {
        setServers(d.items || []);
        if (d.items?.[0]) setDraft((d0) => ({ ...d0, serverId: d.items[0].id }));
      })
      .catch(() => setServers([]));
  }, []);

  useEffect(() => {
    if (!conns.some((c) => c.provider === draft.provider)) {
      setRepos([]);
      return;
    }
    fetch(`/api/git/repos?provider=${encodeURIComponent(draft.provider)}`)
      .then((r) => r.json() as Promise<ListResponse<GitRepo>>)
      .then((d) => setRepos(d.items || []))
      .catch(() => setRepos([]));
  }, [draft.provider, conns]);

  async function inspectRepo(repo: GitRepo) {
    setBusy(true);
    setError(null);
    try {
      const q = new URLSearchParams({ provider: draft.provider, repo: repo.full_name, branch: repo.default_branch });
      const res = await fetch(`/api/git/inspect?${q}`);
      if (!res.ok) throw new Error(await res.text());
      const inspect = (await res.json()) as DeployInspect;
      setDraft((d) => ({
        ...d,
        repo,
        inspect,
        language: inspect.language || "unknown",
        install: inspect.install_script,
        root: inspect.root_directory || ".",
        clonePath: inspect.clone_path,
        branch: inspect.default_branch || repo.default_branch,
        processManager: inspect.process_manager === "pm2" ? "pm2" : "none",
        selectedApps: inspect.workspaces || [],
      }));
      setStep(1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const apps: DeployApp[] = useMemo(() => {
    return draft.selectedApps.map((root) => ({
      name: root.split("/").filter(Boolean).pop() || root,
      root,
      language: draft.language,
      install: draft.install,
      process_manager: draft.processManager,
    }));
  }, [draft.selectedApps, draft.language, draft.install, draft.processManager]);

  const env = useMemo(() => {
    const out: Record<string, string> = {};
    for (const line of draft.envText.split("\n")) {
      const i = line.indexOf("=");
      if (i <= 0) continue;
      out[line.slice(0, i).trim()] = line.slice(i + 1).trim();
    }
    return out;
  }, [draft.envText]);

  async function submit() {
    if (!draft.repo || !draft.serverId) return;
    setBusy(true);
    setError(null);
    try {
      const body = {
        name: draft.repo.full_name,
        provider: draft.provider,
        repo_full_name: draft.repo.full_name,
        repo_id: draft.repo.id,
        clone_url: draft.inspect?.clone_url || draft.repo.clone_url,
        default_branch: draft.branch,
        server_id: draft.serverId,
        language: draft.language,
        install_script: draft.install,
        root_directory: draft.root,
        clone_path: draft.clonePath,
        port: draft.port ? Number(draft.port) : 0,
        process_manager: draft.processManager,
        env,
        apps,
      };
      const res = await fetch("/api/deploys", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = (await res.json()) as { project: DeployProject; run: DeployRun; warnings?: string[] };
      setCreated(data);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (created) {
    return (
      <>
        <Link href="/deploys" className="back-link"><IconChevronLeft /> Back to deploys</Link>
        <div className="page-head">
          <div>
            <h1>Deploy created</h1>
            <p className="subtle">{created.project.repo_full_name} is cloning onto the agent.</p>
          </div>
        </div>
        {created.project.deploy_token && (
          <div className="panel" style={{ maxWidth: 640 }}>
            <div style={{ fontWeight: 700, marginBottom: 8 }}>GitHub Actions token</div>
            <p className="subtle">Store this as <code>CRONCOMPOSE_TOKEN</code>. It is shown once.</p>
            <code className="term-log" style={{ display: "block", padding: 12 }}>{created.project.deploy_token}</code>
          </div>
        )}
        {(created.warnings || []).length > 0 && (
          <div className="panel" style={{ maxWidth: 640 }}>
            <div style={{ fontWeight: 700, marginBottom: 8 }}>Remote setup warnings</div>
            <p className="subtle">The project was created. Webhook or repo files could not be written automatically.</p>
            <ul>
              {created.warnings?.map((w) => <li key={w} className="form-error">{w}</li>)}
            </ul>
          </div>
        )}
        <div className="cluster">
          <Link href={`/deploys/runs/${created.run.id}`} className="button">Watch install</Link>
          <Link href={`/deploys/${created.project.id}`} className="button secondary">Project</Link>
        </div>
      </>
    );
  }

  const connected = conns.some((c) => c.provider === draft.provider);

  return (
    <>
      <Link href="/deploys" className="back-link"><IconChevronLeft /> Back to deploys</Link>
      <div className="page-head">
        <div>
          <h1>Import git</h1>
          <p className="subtle">Clone onto a selected agent, then run the install script.</p>
        </div>
      </div>

      <div className="wizard">
        <Stepper steps={STEPS} current={step} />
        <div className="wizard-body">
          {step === 0 && (
            <>
              <h2 className="step-h">Repository</h2>
              <p className="step-lead">Connect GitHub or GitLab, then pick a repo.</p>
              {!conns.length && (
                <GitConnections initial={[]} next="/app/deploys/new" />
              )}
              {conns.length > 0 && (
                <>
                  <div className="field">
                    <label htmlFor="provider">Provider</label>
                    <select
                      id="provider"
                      value={draft.provider}
                      onChange={(e) => setDraft((d) => ({ ...d, provider: e.target.value, repo: null }))}
                    >
                      {conns.map((c) => (
                        <option key={c.provider} value={c.provider}>{c.provider} ({c.login})</option>
                      ))}
                    </select>
                  </div>
                  {!connected && <p className="subtle">Connect this provider in Settings first.</p>}
                  <div className="stack" style={{ maxHeight: 360, overflow: "auto" }}>
                    {repos.map((r) => (
                      <button
                        type="button"
                        key={r.id}
                        className="panel"
                        style={{ textAlign: "left", width: "100%" }}
                        disabled={busy}
                        onClick={() => inspectRepo(r)}
                      >
                        <div style={{ fontWeight: 700 }}>{r.full_name}</div>
                        <div className="subtle" style={{ fontSize: 13 }}>{r.description || r.default_branch}</div>
                      </button>
                    ))}
                    {connected && repos.length === 0 && <p className="subtle">No repositories visible to this grant.</p>}
                  </div>
                </>
              )}
            </>
          )}

          {step === 1 && (
            <>
              <h2 className="step-h">Build</h2>
              <p className="step-lead">Detected from the repo. Override anything that looks wrong.</p>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="language">Language</label>
                  <input id="language" value={draft.language} onChange={(e) => setDraft((d) => ({ ...d, language: e.target.value }))} />
                </div>
                <div className="field">
                  <label htmlFor="root">Root directory</label>
                  <input id="root" value={draft.root} onChange={(e) => setDraft((d) => ({ ...d, root: e.target.value }))} />
                </div>
              </div>
              <div className="field">
                <label htmlFor="install">Install script</label>
                <textarea id="install" rows={3} value={draft.install} onChange={(e) => setDraft((d) => ({ ...d, install: e.target.value }))} />
              </div>
              <div className="field">
                <label htmlFor="clonePath">Clone path on the agent</label>
                <input id="clonePath" value={draft.clonePath} onChange={(e) => setDraft((d) => ({ ...d, clonePath: e.target.value }))} />
              </div>
              {(draft.inspect?.workspaces || []).length > 0 && (
                <div className="field">
                  <label>Packages (cloned once)</label>
                  {(draft.inspect?.workspaces || []).map((w) => (
                    <label key={w} style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 8 }}>
                      <input
                        type="checkbox"
                        checked={draft.selectedApps.includes(w)}
                        onChange={(e) => setDraft((d) => ({
                          ...d,
                          selectedApps: e.target.checked
                            ? [...d.selectedApps, w]
                            : d.selectedApps.filter((x) => x !== w),
                        }))}
                      />
                      <code>{w}</code>
                    </label>
                  ))}
                </div>
              )}
            </>
          )}

          {step === 2 && (
            <>
              <h2 className="step-h">Runtime</h2>
              <p className="step-lead">Where it runs after clone, and how it is hosted.</p>
              <div className="field">
                <label htmlFor="server">Agent server</label>
                <select id="server" value={draft.serverId} onChange={(e) => setDraft((d) => ({ ...d, serverId: e.target.value }))}>
                  {servers.map((s) => (
                    <option key={s.id} value={s.id}>{s.name} ({s.status})</option>
                  ))}
                </select>
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="branch">Branch</label>
                  <input id="branch" value={draft.branch} onChange={(e) => setDraft((d) => ({ ...d, branch: e.target.value }))} />
                </div>
                <div className="field">
                  <label htmlFor="port">PORT (optional)</label>
                  <input id="port" inputMode="numeric" value={draft.port} onChange={(e) => setDraft((d) => ({ ...d, port: e.target.value }))} />
                </div>
              </div>
              <div className="field">
                <label htmlFor="pm">Process manager</label>
                <select id="pm" value={draft.processManager} onChange={(e) => setDraft((d) => ({ ...d, processManager: e.target.value }))}>
                  <option value="none">None — attach later</option>
                  <option value="pm2">PM2</option>
                  <option value="systemd">systemd (user unit)</option>
                  <option value="docker">Docker Compose</option>
                </select>
                {draft.processManager === "pm2" && draft.inspect && !draft.inspect.has_pm2_ecosystem && (
                  <p className="subtle">No ecosystem file. The agent will run <code>pm2 start npm -- start</code> for Node apps.</p>
                )}
              </div>
              <div className="field">
                <label htmlFor="env">Env vars (KEY=value, one per line)</label>
                <textarea id="env" rows={4} value={draft.envText} onChange={(e) => setDraft((d) => ({ ...d, envText: e.target.value }))} />
              </div>
            </>
          )}

          {step === 3 && (
            <>
              <h2 className="step-h">Review</h2>
              <p className="step-lead">Clone starts as soon as you confirm.</p>
              <div className="stack">
                <div><span className="subtle">Repo</span> {draft.repo?.full_name}</div>
                <div><span className="subtle">Server</span> {servers.find((s) => s.id === draft.serverId)?.name}</div>
                <div><span className="subtle">Path</span> <code>{draft.clonePath}</code></div>
                <div><span className="subtle">Install</span> <code>{draft.install || "(none)"}</code></div>
                <div><span className="subtle">Process</span> {draft.processManager}</div>
              </div>
            </>
          )}

          {error && <p className="form-error">{error}</p>}

          <div className="wizard-foot">
            <button type="button" className="button secondary" disabled={step === 0} onClick={() => setStep((s) => s - 1)}>
              <IconChevronLeft /> Back
            </button>
            {step < 3 ? (
              <button
                type="button"
                className="button"
                disabled={step === 0 || (step === 2 && !draft.serverId)}
                onClick={() => setStep((s) => s + 1)}
              >
                Continue <IconChevronRight />
              </button>
            ) : (
              <button type="button" className="button" disabled={busy || !draft.repo || !draft.serverId} onClick={submit}>
                {busy ? "Starting…" : <><IconCheck /> Deploy</>}
              </button>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
