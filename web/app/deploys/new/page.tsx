"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Stepper, type StepDef } from "@/components/jobwizard/Stepper";
import { GitConnections } from "@/components/deploys/GitConnections";
import { AppEnvEditor } from "@/components/deploys/AppEnvEditor";
import { ProjectBlockCard } from "@/components/deploys/ProjectBlockCard";
import { IconChevronLeft, IconChevronRight, IconCheck } from "@/components/icons";
import {
  blocksReady,
  blocksToDeployApps,
  emptyBlock,
  ensureUniqueBlockNames,
  hasDuplicateRoots,
  seedBlockFromInspect,
  type ProjectBlock,
} from "@/lib/project-blocks";
import type {
  DeployApp,
  DeployEnvVar,
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
  { title: "Build", desc: "Projects to deploy" },
  { title: "Runtime", desc: "Server and env" },
  { title: "Review", desc: "Clone and install" },
];

type Draft = {
  provider: string;
  repo: GitRepo | null;
  inspect: DeployInspect | null;
  serverId: string;
  blocks: ProjectBlock[];
  clonePath: string;
  branch: string;
  appEnv: Record<string, DeployEnvVar[]>;
};

const empty: Draft = {
  provider: "github",
  repo: null,
  inspect: null,
  serverId: "",
  blocks: [],
  clonePath: "",
  branch: "main",
  appEnv: {},
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
        blocks: [seedBlockFromInspect(inspect, repo.full_name)],
        clonePath: inspect.clone_path,
        branch: inspect.default_branch || repo.default_branch,
      }));
      setStep(1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const apps: DeployApp[] = useMemo(
    () => blocksToDeployApps(ensureUniqueBlockNames(draft.blocks), draft.appEnv),
    [draft.blocks, draft.appEnv],
  );

  const duplicateRoots = hasDuplicateRoots(draft.blocks);
  const buildReady = blocksReady(draft.blocks) && !duplicateRoots;

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
        language: draft.blocks[0]?.language || "",
        install_script: draft.blocks[0]?.install || "",
        root_directory: draft.blocks[0]?.root || ".",
        clone_path: draft.clonePath,
        port: draft.blocks[0]?.port ? Number(draft.blocks[0].port) : 0,
        process_manager: draft.blocks[0]?.processManager || "none",
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
  const workspaces = draft.inspect?.workspaces || [];

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

          {step === 1 && draft.repo && (
            <>
              <h2 className="step-h">Build</h2>
              <p className="step-lead">Configure each app to deploy from this repo.</p>
              <div className="stack">
                {draft.blocks.map((block) => (
                  <ProjectBlockCard
                    key={block.id}
                    block={block}
                    workspaces={workspaces}
                    provider={draft.provider}
                    repo={draft.repo!.full_name}
                    branch={draft.branch}
                    canRemove={draft.blocks.length > 1}
                    onChange={(next) =>
                      setDraft((d) => ({
                        ...d,
                        blocks: d.blocks.map((b) => (b.id === next.id ? next : b)),
                      }))
                    }
                    onRemove={() =>
                      setDraft((d) => ({
                        ...d,
                        blocks: d.blocks.filter((b) => b.id !== block.id),
                      }))
                    }
                  />
                ))}
              </div>
              <button
                type="button"
                className="button secondary"
                style={{ marginTop: 12 }}
                onClick={() => setDraft((d) => ({ ...d, blocks: [...d.blocks, emptyBlock()] }))}
              >
                + Add project
              </button>
              {duplicateRoots && (
                <p className="form-error" style={{ marginTop: 12 }}>
                  Two projects share the same root folder. Pick a different folder for each.
                </p>
              )}
              {!blocksReady(draft.blocks) && draft.blocks.length > 0 && (
                <p className="form-error" style={{ marginTop: 12 }}>
                  Every project needs a root folder before continuing.
                </p>
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
                  <input
                    id="port"
                    inputMode="numeric"
                    value={draft.blocks[0]?.port ?? ""}
                    onChange={(e) =>
                      setDraft((d) => ({
                        ...d,
                        blocks: d.blocks.map((b, i) => (i === 0 ? { ...b, port: e.target.value } : b)),
                      }))
                    }
                  />
                </div>
              </div>
              <div className="field">
                <label htmlFor="pm">Process manager</label>
                <select
                  id="pm"
                  value={draft.blocks[0]?.processManager ?? "none"}
                  onChange={(e) =>
                    setDraft((d) => ({
                      ...d,
                      blocks: d.blocks.map((b, i) => (i === 0 ? { ...b, processManager: e.target.value } : b)),
                    }))
                  }
                >
                  <option value="none">None — attach later</option>
                  <option value="pm2">PM2</option>
                  <option value="systemd">systemd (user unit)</option>
                  <option value="docker">Docker Compose</option>
                </select>
                {draft.blocks[0]?.processManager === "pm2" && draft.inspect && !draft.inspect.has_pm2_ecosystem && (
                  <p className="subtle">No ecosystem file. The agent will run <code>pm2 start npm -- start</code> for Node apps.</p>
                )}
              </div>
              <AppEnvEditor
                apps={apps}
                onChange={(next) => {
                  const appEnv: Record<string, DeployEnvVar[]> = {};
                  for (const a of next) appEnv[a.name] = a.env ?? [];
                  setDraft((d) => ({ ...d, appEnv }));
                }}
              />
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
                <div><span className="subtle">Apps</span> {draft.blocks.length}</div>
                <div><span className="subtle">Install</span> <code>{draft.blocks[0]?.install || "(none)"}</code></div>
                <div><span className="subtle">Process</span> {draft.blocks[0]?.processManager}</div>
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
                disabled={
                  step === 0
                  || (step === 1 && !buildReady)
                  || (step === 2 && !draft.serverId)
                }
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
