"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { DeployConfigTabs } from "@/components/deploys/DeployConfigTabs";
import { RedeployButton } from "@/components/deploys/RedeployButton";
import { IconChevronLeft, IconDownload } from "@/components/icons";
import {
  blocksFromProject,
  blocksReady,
  blocksToDeployApps,
  ensureUniqueBlockNames,
  hasDuplicateRoots,
  type ProjectBlock,
  type RedeployMode,
} from "@/lib/project-blocks";
import {
  draftToYaml,
  downloadYaml,
  emptyAdvanced,
  type AdvancedSettings,
} from "@/lib/deploy-spec";
import {
  blocksWithStartGroupsFromFlow,
  defaultFlow,
  flowFromSpec,
  flowProblems,
  flowSignature,
  flowToSpec,
  reconcileFlow,
  type FlowWave,
} from "@/lib/deploy-flow";
import { listBranches } from "@/lib/git-detect";
import type { SelectOption } from "@/lib/ui-helpers";
import type { DeployApp, DeployEnvVar, DeployProject } from "@/lib/types";

type Detail = { project: DeployProject };

export default function EditDeployPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params.id;

  const [project, setProject] = useState<DeployProject | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);

  const [name, setName] = useState("");
  const [branch, setBranch] = useState("main");
  const [clonePath, setClonePath] = useState("");
  const [runAsUser, setRunAsUser] = useState("");
  const [blocks, setBlocks] = useState<ProjectBlock[]>([]);
  const [flow, setFlow] = useState<FlowWave[]>([]);
  const [globalEnv, setGlobalEnv] = useState<DeployEnvVar[]>([]);
  const [appEnv, setAppEnv] = useState<Record<string, DeployEnvVar[]>>({});
  const [advanced, setAdvanced] = useState<AdvancedSettings>({ ...emptyAdvanced });
  const [redeployOn, setRedeployOn] = useState<RedeployMode[]>(["branch"]);
  const [branches, setBranches] = useState<SelectOption[]>([]);
  const [branchesBusy, setBranchesBusy] = useState(false);

  useEffect(() => {
    let live = true;
    fetch(`/api/deploys/${id}`)
      .then(async (r) => {
        if (!r.ok) throw new Error(await r.text());
        return r.json() as Promise<Detail>;
      })
      .then((d) => {
        if (!live) return;
        const p = d.project;
        setProject(p);
        setName(p.name);
        setBranch(p.default_branch);
        setClonePath(p.clone_path);
        setRunAsUser(p.run_as_user || "");
        const nextBlocks = blocksFromProject(p);
        setBlocks(nextBlocks);
        const fromSpec = flowFromSpec(nextBlocks, p.flow ?? null);
        setFlow(fromSpec ?? defaultFlow(nextBlocks));
        setGlobalEnv(
          Object.entries(p.env || {}).map(([key, value]) => ({
            key,
            value,
            sensitive: false,
            has_value: true,
          })),
        );
        const envMap: Record<string, DeployEnvVar[]> = {};
        for (const app of p.apps || []) {
          if (app.env?.length) envMap[app.name] = app.env;
        }
        setAppEnv(envMap);
        setAdvanced({
          ...emptyAdvanced,
          autoRollback: p.auto_rollback,
          onWaveFailure: p.on_wave_failure === "continue" ? "continue" : "stop",
          healthPath: p.health_path || "",
          healthPort: p.health_port ? String(p.health_port) : "",
          healthTimeout: p.health_timeout_seconds ? String(p.health_timeout_seconds) : "",
          deployTimeout: p.deploy_timeout_seconds ? String(p.deploy_timeout_seconds) : "",
        });
        const modes = (p.redeploy_on || []).filter((m): m is RedeployMode =>
          m === "branch" || m === "tag" || m === "release",
        );
        setRedeployOn(modes.length ? modes : ["branch"]);
      })
      .catch((e) => live && setError((e as Error).message));
    return () => {
      live = false;
    };
  }, [id]);

  useEffect(() => {
    if (!project) return;
    if (project.provider !== "github" && project.provider !== "gitlab") {
      setBranches([]);
      setBranchesBusy(false);
      return;
    }
    let live = true;
    setBranchesBusy(true);
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
      .catch(() => live && setBranches([]))
      .finally(() => {
        if (live) setBranchesBusy(false);
      });
    return () => {
      live = false;
    };
  }, [project]);

  // Keep flow tasks in sync when processes / scripts change (old deploys get a default).
  useEffect(() => {
    if (blocks.length === 0) return;
    setFlow((prev) => {
      const base = prev.length ? prev : defaultFlow(blocks);
      const next = reconcileFlow(blocks, base);
      return flowSignature(next) === flowSignature(prev) ? prev : next;
    });
  }, [
    blocks
      .map((b) => `${b.id}:${b.install}:${b.cleanup}:${b.preStart}:${b.processManager}:${b.healthPath}`)
      .join("|"),
  ]);

  const orderedBlocks = useMemo(
    () => blocksWithStartGroupsFromFlow(ensureUniqueBlockNames(blocks), flow),
    [blocks, flow],
  );

  const apps: DeployApp[] = useMemo(
    () => blocksToDeployApps(orderedBlocks, appEnv),
    [orderedBlocks, appEnv],
  );

  const problems = useMemo(() => {
    const out: string[] = [];
    if (!name.trim()) out.push("Name is required");
    if (!blocksReady(blocks)) out.push("Each process needs a root folder");
    if (hasDuplicateRoots(blocks)) out.push("Two processes share the same root");
    out.push(...flowProblems(flow));
    return out;
  }, [name, blocks, flow]);

  const exportYaml = useMemo(() => {
    if (!project) return "";
    return draftToYaml({
      name: name.trim() || project.name,
      provider: project.provider,
      repo: project.repo_full_name,
      branch: branch || project.default_branch,
      server: project.server_id,
      clonePath: clonePath || undefined,
      blocks: orderedBlocks,
      globalEnv,
      appEnv,
      advanced,
      redeployOn,
      flow: flowToSpec(orderedBlocks, flow),
    });
  }, [project, name, branch, clonePath, orderedBlocks, globalEnv, appEnv, advanced, redeployOn, flow]);

  function shareYaml() {
    if (!project || !exportYaml) return;
    const safe = (name || project.name || "deploy").replace(/[^a-zA-Z0-9._-]+/g, "-").replace(/^-|-$/g, "") || "deploy";
    downloadYaml(`croncompose-${safe}.yml`, exportYaml);
  }

  async function save() {
    if (!project || problems.length) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${project.id}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          default_branch: branch,
          install_script: blocks[0]?.install || project.install_script,
          clone_path: clonePath,
          run_as_user: runAsUser,
          port: blocks[0]?.port ? Number(blocks[0].port) : 0,
          process_manager: blocks[0]?.processManager || project.process_manager,
          auto_rollback: advanced.autoRollback,
          on_wave_failure: advanced.onWaveFailure,
          health_path: advanced.healthPath.trim(),
          health_port: advanced.healthPort ? Number(advanced.healthPort) : 0,
          health_timeout_seconds: advanced.healthTimeout ? Number(advanced.healthTimeout) : 60,
          deploy_timeout_seconds: advanced.deployTimeout ? Number(advanced.deployTimeout) : 0,
          redeploy_on: redeployOn,
          apps,
          flow: flowToSpec(orderedBlocks, flow),
          env: Object.fromEntries(
            globalEnv.filter((e) => e.key.trim()).map((e) => [e.key.trim(), e.value || ""]),
          ),
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      setSaved(true);
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (error && !project) {
    return (
      <>
        <Link href="/deploys" className="back-link"><IconChevronLeft /> Deploy</Link>
        <div className="form-error">Could not load project: <code>{error}</code></div>
      </>
    );
  }

  if (!project) {
    return <p className="subtle">Loading…</p>;
  }

  return (
    <div className="deploy-flow wide">
      <Link href={`/deploys/${project.id}`} className="back-link">
        <IconChevronLeft /> Back to project
      </Link>

      <div className="page-head">
        <div>
          <h1>Edit project</h1>
          <p className="subtle">
            Same configure view as create. {project.provider}/{project.repo_full_name}
          </p>
        </div>
      </div>

      <DeployConfigTabs
        projectName={name}
        onProjectName={setName}
        branch={branch}
        onBranch={setBranch}
        branches={branches}
        provider={project.provider}
        repo={project.repo_full_name}
        showBranch={project.provider === "github" || project.provider === "gitlab"}
        showSpecPicker={false}
        specFileOptions={[]}
        selectedSpecPath=""
        onSelectSpecPath={() => {}}
        specBusy={false}
        blocks={blocks}
        onBlocks={setBlocks}
        apps={apps}
        globalEnv={globalEnv}
        onGlobalEnv={setGlobalEnv}
        appEnv={appEnv}
        onAppEnv={setAppEnv}
        advanced={advanced}
        onAdvanced={setAdvanced}
        clonePath={clonePath}
        onClonePath={setClonePath}
        serverId={project.server_id}
        runAsUser={runAsUser}
        onRunAsUser={setRunAsUser}
        inspect={null}
        redeployOn={redeployOn}
        onRedeployOn={setRedeployOn}
        gitConnected
        branchesBusy={branchesBusy}
        flow={flow}
        onFlow={setFlow}
      />

      {error && <p className="form-error">{error}</p>}

      <div className="deploy-bar">
        <button type="button" className="button ghost sm" onClick={shareYaml}>
          <IconDownload /> Export croncompose.yml
        </button>
        <span className="subtle deploy-bar-hint">
          {problems[0] || (saved ? "Saved. Redeploy to apply on the server." : "Save updates the project; redeploy applies them.")}
        </span>
        <div className="deploy-bar-right">
          {saved && (
            <RedeployButton
              projectId={project.id}
              provider={project.provider}
              repo={project.repo_full_name}
              defaultBranch={branch || project.default_branch}
              compact
            />
          )}
          <button
            type="button"
            className="button deploy-button"
            disabled={busy || problems.length > 0}
            onClick={() => void save()}
          >
            {busy ? "Saving…" : "Save changes"}
          </button>
        </div>
      </div>
    </div>
  );
}
