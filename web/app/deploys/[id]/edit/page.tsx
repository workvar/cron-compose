"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { DeployConfigTabs } from "@/components/deploys/DeployConfigTabs";
import { RedeployButton } from "@/components/deploys/RedeployButton";
import { IconChevronLeft } from "@/components/icons";
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
  emptyAdvanced,
  type AdvancedSettings,
} from "@/lib/deploy-spec";
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
  const [globalEnv, setGlobalEnv] = useState<DeployEnvVar[]>([]);
  const [appEnv, setAppEnv] = useState<Record<string, DeployEnvVar[]>>({});
  const [advanced, setAdvanced] = useState<AdvancedSettings>({ ...emptyAdvanced });
  const [redeployOn, setRedeployOn] = useState<RedeployMode[]>(["branch"]);
  const [branches, setBranches] = useState<SelectOption[]>([]);

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
        setBlocks(blocksFromProject(p));
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
      .catch(() => live && setBranches([]));
    return () => {
      live = false;
    };
  }, [project]);

  const apps: DeployApp[] = useMemo(
    () => blocksToDeployApps(ensureUniqueBlockNames(blocks), appEnv),
    [blocks, appEnv],
  );

  const problems = useMemo(() => {
    const out: string[] = [];
    if (!name.trim()) out.push("Name is required");
    if (!blocksReady(blocks)) out.push("Each process needs a root folder");
    if (hasDuplicateRoots(blocks)) out.push("Two processes share the same root");
    return out;
  }, [name, blocks]);

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
          health_path: advanced.healthPath.trim(),
          health_port: advanced.healthPort ? Number(advanced.healthPort) : 0,
          health_timeout_seconds: advanced.healthTimeout ? Number(advanced.healthTimeout) : 60,
          deploy_timeout_seconds: advanced.deployTimeout ? Number(advanced.deployTimeout) : 0,
          redeploy_on: redeployOn,
          apps,
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
      />

      {error && <p className="form-error">{error}</p>}

      <div className="deploy-bar">
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
