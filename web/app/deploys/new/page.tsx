"use client";

// Vercel-style import: pick a source (connected repo, public URL, or a pasted
// croncompose.yml), configure (including which repo croncompose.yml to use),
// review the steps that YAML implies, then confirm.
import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { GitConnections } from "@/components/deploys/GitConnections";
import { DeployConfigTabs } from "@/components/deploys/DeployConfigTabs";
import { DeployReview } from "@/components/deploys/DeployReview";
import { DeployServerChip } from "@/components/deploys/DeployServerChip";
import { ServerPickerGrid } from "@/components/deploys/ServerPickerGrid";
import CopyButton from "@/components/CopyButton";
import {
  IconCheck,
  IconChevronLeft,
  IconDownload,
  IconGit,
  IconSearch,
} from "@/components/icons";
import {
  blocksReady,
  blocksToDeployApps,
  emptyBlock,
  ensureUniqueBlockNames,
  hasDuplicateRoots,
  seedBlockFromInspect,
  DEFAULT_REDEPLOY_ON,
  type ProjectBlock,
  type RedeployMode,
} from "@/lib/project-blocks";
import {
  SPEC_TEMPLATE,
  draftToYaml,
  emptyAdvanced,
  envVarsToRecord,
  parseRepoUrl,
  specToDraft,
  type AdvancedSettings,
} from "@/lib/deploy-spec";
import { buildDeployPlan } from "@/lib/deploy-steps";
import { filterGitRepos, listGitRepoOwners, toggleOwnerFilter } from "@/lib/git-repos";
import { fetchSpecFile, listBranches, listSpecFiles } from "@/lib/git-detect";
import { githubLanguageIconUrl } from "@/lib/language-icons";
import type { SelectOption } from "@/lib/ui-helpers";
import type {
  DeployApp,
  DeployEnvVar,
  DeployInspect,
  DeployProject,
  DeployRun,
  DeploySpecIssue,
  DeploySpecResult,
  GitConnection,
  GitRepo,
  ListResponse,
  Server,
} from "@/lib/types";

/** Sentinel for "don't use a croncompose.yml; detect from the repo". */
const SPEC_NONE = "__none__";

type Source = {
  provider: string;
  fullName: string;
  repoId?: string;
  cloneUrl: string;
  defaultBranch: string;
  /** How the repo was picked; "public" and "yaml" may have no git grant. */
  via: "git" | "public" | "yaml";
};

type Form = {
  name: string;
  serverId: string;
  branch: string;
  clonePath: string;
  blocks: ProjectBlock[];
  globalEnv: DeployEnvVar[];
  appEnv: Record<string, DeployEnvVar[]>;
  advanced: AdvancedSettings;
  redeployOn: RedeployMode[];
};

const emptyForm: Form = {
  name: "",
  serverId: "",
  branch: "main",
  clonePath: "",
  blocks: [],
  globalEnv: [],
  appEnv: {},
  advanced: emptyAdvanced,
  redeployOn: [...DEFAULT_REDEPLOY_ON],
};

async function errorText(res: Response): Promise<string> {
  const raw = await res.text();
  try {
    const j = JSON.parse(raw) as { error?: { message?: string } };
    return j.error?.message || raw;
  } catch {
    return raw || `HTTP ${res.status}`;
  }
}

export default function NewDeployPage() {
  const [phase, setPhase] = useState<"server" | "import" | "configure" | "review">("server");
  const [conns, setConns] = useState<GitConnection[] | null>(null);
  const [provider, setProvider] = useState("github");
  const [repos, setRepos] = useState<GitRepo[] | null>(null);
  const [repoQuery, setRepoQuery] = useState("");
  const [ownerFilter, setOwnerFilter] = useState<string[]>([]);
  const [servers, setServers] = useState<Server[]>([]);

  const [publicUrl, setPublicUrl] = useState("");
  const [yamlText, setYamlText] = useState("");
  const [yamlIssues, setYamlIssues] = useState<DeploySpecIssue[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);

  const [source, setSource] = useState<Source | null>(null);
  const [branches, setBranches] = useState<SelectOption[]>([]);
  const [inspect, setInspect] = useState<DeployInspect | null>(null);
  const [spec, setSpec] = useState<DeploySpecResult | null>(null);
  const [specFiles, setSpecFiles] = useState<string[]>([]);
  const [selectedSpecPath, setSelectedSpecPath] = useState(SPEC_NONE);
  const [form, setForm] = useState<Form>(emptyForm);
  const [showYaml, setShowYaml] = useState(false);

  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<{ project: DeployProject; run: DeployRun; warnings?: string[] } | null>(null);

  useEffect(() => {
    fetch("/api/git/connections")
      .then((r) => r.json() as Promise<ListResponse<GitConnection>>)
      .then((d) => {
        setConns(d.items || []);
        if (d.items?.[0]) setProvider(d.items[0].provider);
      })
      .catch(() => setConns([]));
    fetch("/api/servers")
      .then((r) => r.json() as Promise<ListResponse<Server>>)
      .then((d) => setServers(d.items || []))
      .catch(() => setServers([]));
  }, []);

  const connected = (conns || []).some((c) => c.provider === provider);

  useEffect(() => {
    setRepoQuery("");
    setOwnerFilter([]);
    if (!connected) {
      setRepos([]);
      return;
    }
    setRepos(null);
    fetch(`/api/git/repos?provider=${encodeURIComponent(provider)}`)
      .then((r) => r.json() as Promise<ListResponse<GitRepo>>)
      .then((d) => setRepos(d.items || []))
      .catch(() => setRepos([]));
  }, [provider, connected]);

  // Branch options for the searchable branch picker, loaded once a repo is open.
  useEffect(() => {
    if (!source) {
      setBranches([]);
      return;
    }
    let live = true;
    listBranches(source.provider, source.fullName)
      .then((items) => {
        if (live) setBranches(items.map((b) => ({ value: b.name, label: b.default ? `${b.name} (default)` : b.name })));
      })
      .catch(() => {
        if (live) setBranches([]);
      });
    return () => {
      live = false;
    };
  }, [source?.provider, source?.fullName]);

  // croncompose.yml files anywhere in the repo, for the file picker.
  useEffect(() => {
    if (!source || source.via === "yaml" || phase === "import") {
      setSpecFiles([]);
      return;
    }
    let live = true;
    listSpecFiles(source.provider, source.fullName, form.branch || source.defaultBranch || "main")
      .then((items) => {
        if (live) setSpecFiles(items);
      })
      .catch(() => {
        if (live) setSpecFiles([]);
      });
    return () => {
      live = false;
    };
  }, [source?.provider, source?.fullName, source?.via, source?.defaultBranch, form.branch, phase]);

  const personalLogin = conns?.find((c) => c.provider === provider)?.login;
  const owners = useMemo(() => listGitRepoOwners(repos || [], personalLogin), [repos, personalLogin]);
  const visibleRepos = useMemo(
    () => filterGitRepos(repos || [], { query: repoQuery, owners: ownerFilter }),
    [repos, repoQuery, ownerFilter],
  );

  // A server is picked for the user when there is only one choice, or it is the
  // only one online, so the common case needs no clicks at all.
  function defaultServer(): string {
    if (servers.length === 1) return servers[0].id;
    const online = servers.filter((s) => s.status === "online");
    return online.length === 1 ? online[0].id : "";
  }

  /** Fetches detection (+ the repo's own croncompose.yml) and opens Configure. */
  async function openRepo(src: Source, pasted: DeploySpecResult | null, key: string) {
    setBusy(key);
    setError(null);
    try {
      const hasGrant = (conns || []).some((c) => c.provider === src.provider);
      const q = new URLSearchParams({ provider: src.provider, repo: src.fullName });
      if (pasted?.spec.branch) q.set("branch", pasted.spec.branch);
      else if (src.defaultBranch) q.set("branch", src.defaultBranch);
      if (!hasGrant) q.set("public", "1");
      let ins: DeployInspect | null = null;
      const res = await fetch(`/api/git/inspect?${q}`);
      if (res.ok) {
        ins = (await res.json()) as DeployInspect;
      } else if (!pasted) {
        // Without a file there is nothing to configure from.
        throw new Error(
          src.via === "public"
            ? `Could not read ${src.fullName}. Is it public? ${await errorText(res)}`
            : await errorText(res),
        );
      }
      const fileSpec = pasted || ins?.spec || null;
      const repoName = src.fullName.split("/").pop() || src.fullName;
      const next: Form = {
        ...emptyForm,
        name: repoName,
        // Keep the server chosen on the grid; only fall back if somehow empty.
        serverId: form.serverId || defaultServer(),
        branch: ins?.default_branch || src.defaultBranch || "main",
        clonePath: ins?.clone_path || "",
        blocks: ins ? [seedBlockFromInspect(ins, src.fullName)] : [emptyBlock()],
      };
      if (!ins) next.blocks[0] = { ...next.blocks[0], root: ".", name: repoName };
      if (fileSpec && fileSpec.valid) {
        const d = specToDraft({ ...fileSpec.spec, repo: fileSpec.spec.repo || src.fullName }, servers, {
          language: ins?.language,
          install: ins?.install_script,
        });
        next.name = d.name || next.name;
        next.branch = d.branch || next.branch;
        next.clonePath = d.clonePath || next.clonePath;
        // Spec may name a server; prefer the operator's grid choice when set.
        next.serverId = form.serverId || d.serverId || next.serverId;
        next.blocks = d.blocks;
        next.globalEnv = d.globalEnv;
        next.appEnv = d.appEnv;
        next.advanced = d.advanced;
        next.redeployOn = d.redeployOn;
      }
      setSource({
        ...src,
        cloneUrl: ins?.clone_url || src.cloneUrl,
        defaultBranch: ins?.default_branch || src.defaultBranch,
      });
      setInspect(ins);
      setSpec(fileSpec);
      // Pasted YAML is not a repo path; root auto-detect is. Nested picks come from the picker.
      if (pasted) setSelectedSpecPath(SPEC_NONE);
      else if (fileSpec?.path) setSelectedSpecPath(fileSpec.path);
      else setSelectedSpecPath(SPEC_NONE);
      setForm(next);
      setShowYaml(false);
      setPhase("configure");
      window.scrollTo({ top: 0 });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  function importRepo(r: GitRepo) {
    void openRepo(
      { provider, fullName: r.full_name, repoId: r.id, cloneUrl: r.clone_url, defaultBranch: r.default_branch, via: "git" },
      null,
      `repo:${r.id}`,
    );
  }

  function importPublic() {
    const ref = parseRepoUrl(publicUrl);
    if (!ref) {
      setError("Paste a GitHub or GitLab URL, like https://github.com/owner/repo");
      return;
    }
    void openRepo(
      { provider: ref.provider, fullName: ref.fullName, cloneUrl: "", defaultBranch: "", via: "public" },
      null,
      "public",
    );
  }

  async function importYaml() {
    setBusy("yaml");
    setError(null);
    setYamlIssues([]);
    try {
      const res = await fetch("/api/deploys/spec/validate", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ yaml: yamlText }),
      });
      if (!res.ok) throw new Error(await errorText(res));
      const result = (await res.json()) as DeploySpecResult;
      setYamlIssues(result.issues);
      if (!result.valid) return;
      if (!result.spec.repo) {
        setYamlIssues([...result.issues, { level: "error", field: "repo", message: "Add repo: (owner/name or a URL) so CronCompose knows what to clone." }]);
        return;
      }
      setBusy(null);
      await openRepo(
        {
          provider: result.spec.provider,
          fullName: result.spec.repo,
          cloneUrl: "",
          defaultBranch: result.spec.branch || "",
          via: "yaml",
        },
        { ...result, path: "pasted croncompose.yml" },
        "yaml",
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  async function loadFile(file: File) {
    setYamlText(await file.text());
    setYamlIssues([]);
  }

  const apps: DeployApp[] = useMemo(
    () => blocksToDeployApps(ensureUniqueBlockNames(form.blocks), form.appEnv),
    [form.blocks, form.appEnv],
  );
  const duplicateRoots = hasDuplicateRoots(form.blocks);
  const problems: string[] = [];
  if (!form.serverId) problems.push("Pick a server to deploy to.");
  if (!blocksReady(form.blocks)) problems.push("Every app needs a root folder.");
  if (duplicateRoots) problems.push("Two apps share the same root folder.");
  if (spec && !spec.valid && selectedSpecPath !== SPEC_NONE) {
    problems.push("Fix the selected croncompose.yml before continuing.");
  }
  const canContinue = problems.length === 0 && !busy;

  const selectedServer = useMemo(
    () => servers.find((s) => s.id === form.serverId) || null,
    [servers, form.serverId],
  );

  const specFileOptions = useMemo<SelectOption[]>(() => {
    const opts: SelectOption[] = [{ value: SPEC_NONE, label: "None — detect from repo" }];
    for (const p of specFiles) opts.push({ value: p, label: p });
    // Keep a selected path visible even if the listing hasn't returned it yet.
    if (selectedSpecPath !== SPEC_NONE && !specFiles.includes(selectedSpecPath)) {
      opts.push({ value: selectedSpecPath, label: selectedSpecPath });
    }
    return opts;
  }, [specFiles, selectedSpecPath]);

  const reviewPlan = useMemo(
    () =>
      source
        ? buildDeployPlan({
            repo: source.fullName,
            branch: form.branch,
            serverName: servers.find((s) => s.id === form.serverId)?.name || "",
            clonePath: form.clonePath,
            blocks: ensureUniqueBlockNames(form.blocks),
            advanced: form.advanced,
            globalEnv: form.globalEnv,
            appEnv: form.appEnv,
          })
        : [],
    [source, form, servers],
  );

  /** Seeds the form from detection only (no croncompose.yml). */
  function formFromInspect(src: Source, ins: DeployInspect | null): Form {
    const repoName = src.fullName.split("/").pop() || src.fullName;
    return {
      ...emptyForm,
      name: repoName,
      // Keep the server / branch the operator already picked when clearing a YAML.
      serverId: form.serverId || defaultServer(),
      branch: form.branch || ins?.default_branch || src.defaultBranch || "main",
      clonePath: ins?.clone_path || "",
      blocks: ins ? [seedBlockFromInspect(ins, src.fullName)] : [emptyBlock()],
    };
  }

  function applySpecToForm(src: Source, ins: DeployInspect | null, fileSpec: DeploySpecResult): Form {
    const next = formFromInspect(src, ins);
    if (!fileSpec.valid) return next;
    const d = specToDraft({ ...fileSpec.spec, repo: fileSpec.spec.repo || src.fullName }, servers, {
      language: ins?.language,
      install: ins?.install_script,
    });
    next.name = d.name || next.name;
    next.branch = d.branch || next.branch;
    next.clonePath = d.clonePath || next.clonePath;
    next.serverId = d.serverId || next.serverId;
    next.blocks = d.blocks;
    next.globalEnv = d.globalEnv;
    next.appEnv = d.appEnv;
    next.advanced = d.advanced;
    next.redeployOn = d.redeployOn;
    return next;
  }

  async function onSelectSpecPath(path: string) {
    if (!source) return;
    setSelectedSpecPath(path);
    setError(null);
    if (path === SPEC_NONE) {
      setSpec(null);
      setForm(formFromInspect(source, inspect));
      return;
    }
    setBusy("spec");
    try {
      const result = await fetchSpecFile(
        source.provider,
        source.fullName,
        form.branch || source.defaultBranch || "main",
        path,
      );
      setSpec(result);
      if (result.valid) setForm(applySpecToForm(source, inspect, result));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  const exportYaml = source
    ? draftToYaml({
        name: form.name,
        provider: source.provider,
        repo: source.fullName,
        branch: form.branch,
        server: servers.find((s) => s.id === form.serverId)?.name,
        clonePath: form.clonePath,
        blocks: ensureUniqueBlockNames(form.blocks),
        globalEnv: form.globalEnv,
        appEnv: form.appEnv,
        advanced: form.advanced,
        redeployOn: form.redeployOn,
      })
    : "";

  async function deploy() {
    if (!source || !canContinue) return;
    setBusy("deploy");
    setError(null);
    try {
      const submitApps = blocksToDeployApps(ensureUniqueBlockNames(form.blocks), form.appEnv);
      const first = submitApps[0];
      const a = form.advanced;
      const body = {
        name: form.name.trim() || source.fullName,
        provider: source.provider,
        repo_full_name: source.fullName,
        repo_id: source.repoId,
        clone_url: source.cloneUrl || undefined,
        default_branch: form.branch.trim() || source.defaultBranch || "main",
        server_id: form.serverId,
        language: first?.language || "",
        install_script: first?.install || "",
        root_directory: first?.root || ".",
        clone_path: form.clonePath.trim(),
        port: first?.port || 0,
        process_manager: first?.process_manager || "none",
        env: envVarsToRecord(form.globalEnv),
        apps: submitApps,
        // A repo croncompose.yml (auto-detected or picked) must not be overwritten.
        spec_from_repo: selectedSpecPath !== SPEC_NONE || !!inspect?.spec,
        auto_rollback: a.autoRollback,
        health_path: a.healthPath.trim(),
        health_port: Number(a.healthPort) || 0,
        health_timeout_seconds: Number(a.healthTimeout) || 0,
        deploy_timeout_seconds: Number(a.deployTimeout) || 0,
        redeploy_on: form.redeployOn,
      };
      const res = await fetch("/api/deploys", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(await errorText(res));
      setCreated((await res.json()) as { project: DeployProject; run: DeployRun; warnings?: string[] });
      window.scrollTo({ top: 0 });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }

  if (created) {
    return (
      <div className="deploy-flow">
        <div className="deploy-done panel">
          <div className="deploy-done-icon"><IconCheck /></div>
          <h1>Deploying {created.project.name}</h1>
          <p className="subtle">
            {created.run.status === "agent_offline"
              ? "The server is offline. The deploy starts when its agent reconnects, or redeploy from the project page."
              : "The agent is cloning the repo and running your install command."}
          </p>
          <div className="cluster" style={{ justifyContent: "center" }}>
            <Link href={`/deploys/runs/${created.run.id}`} className="button">View build logs</Link>
            <Link href={`/deploys/${created.project.id}`} className="button secondary">Go to project</Link>
          </div>
        </div>
        {created.project.deploy_token && (
          <div className="panel">
            <div className="card-title">CI deploy token</div>
            <p className="subtle">
              Save this as a <code>CRONCOMPOSE_TOKEN</code> secret in your repo to deploy from CI. It is shown only once.
            </p>
            <div className="cluster" style={{ flexWrap: "nowrap" }}>
              <code className="spec-code" style={{ flex: 1 }}>{created.project.deploy_token}</code>
              <CopyButton value={created.project.deploy_token} className="light" />
            </div>
          </div>
        )}
        {(created.warnings || []).length > 0 && (
          <div className="panel">
            <div className="card-title">Heads up</div>
            <ul className="issue-list">
              {created.warnings?.map((w) => <li key={w} className="issue warning">{w}</li>)}
            </ul>
          </div>
        )}
      </div>
    );
  }

  if (phase === "review" && source) {
    const serverName = selectedServer?.name || "";
    const reviewSpecPath =
      selectedSpecPath !== SPEC_NONE
        ? selectedSpecPath
        : source.via === "yaml"
          ? "pasted croncompose.yml"
          : spec?.path;
    return (
      <div className="deploy-flow">
        <button type="button" className="back-link as-button" onClick={() => setPhase("configure")}>
          <IconChevronLeft /> Back
        </button>
        <div className="page-head">
          <div>
            <h1>Confirm deploy</h1>
            <div className="repo-chip">
              <IconGit />
              <span>{source.fullName}</span>
              <span className="pill">{form.branch || "main"}</span>
            </div>
          </div>
        </div>

        <DeployServerChip server={selectedServer} onChange={() => setPhase("server")} />

        <DeployReview
          repo={source.fullName}
          branch={form.branch}
          serverName={serverName}
          specPath={reviewSpecPath}
          plan={reviewPlan}
          issues={spec?.issues}
        />

        {error && <p className="form-error">{error}</p>}

        <div className="deploy-bar">
          <button type="button" className="button ghost sm" onClick={() => setPhase("configure")}>
            Edit configuration
          </button>
          <div className="deploy-bar-right">
            {problems.length > 0 && <span className="subtle deploy-bar-hint">{problems[0]}</span>}
            <button type="button" className="button deploy-button" disabled={!canContinue} onClick={deploy}>
              {busy === "deploy" ? "Deploying…" : "Confirm & deploy"}
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (phase === "server") {
    return (
      <div className="deploy-flow wide">
        <Link href="/deploys" className="back-link"><IconChevronLeft /> Back to deploys</Link>
        <div className="page-head">
          <div>
            <h1>Choose a server</h1>
            <p className="subtle">
              Pick where this code will run. You can assign a green emoji on the server page for a
              quick visual identifier.
            </p>
          </div>
        </div>
        <ServerPickerGrid
          servers={servers}
          value={form.serverId}
          onChange={(serverId) => setForm((f) => ({ ...f, serverId }))}
          onContinue={() => {
            setError(null);
            setPhase(source ? "configure" : "import");
            window.scrollTo({ top: 0 });
          }}
        />
      </div>
    );
  }

  if (phase === "configure" && source) {
    const showSpecPicker = source.via !== "yaml";
    const gitConnected = (conns || []).some((c) => c.provider === source.provider);
    return (
      <div className="deploy-flow wide">
        <button type="button" className="back-link as-button" onClick={() => setPhase("import")}>
          <IconChevronLeft /> Back
        </button>
        <div className="page-head">
          <div>
            <h1>Configure project</h1>
            <div className="repo-chip">
              <IconGit />
              <span>{source.fullName}</span>
              <span className="pill">{form.branch || "main"}</span>
              {source.via === "public" && <span className="pill">public</span>}
            </div>
          </div>
        </div>

        <DeployServerChip server={selectedServer} onChange={() => setPhase("server")} />

        {spec && selectedSpecPath !== SPEC_NONE && (
          <div className={`spec-banner${spec.valid ? " ok" : " bad"}`}>
            <div>
              <strong>
                {spec.valid
                  ? `Configured from ${spec.path || "croncompose.yml"}`
                  : `${spec.path || "croncompose.yml"} has errors and was not applied`}
              </strong>
              <span className="subtle"> · you can still change anything below.</span>
            </div>
            {spec.issues.length > 0 && <IssueList issues={spec.issues} />}
          </div>
        )}
        {source.via === "yaml" && spec && (
          <div className={`spec-banner${spec.valid ? " ok" : " bad"}`}>
            <div>
              <strong>
                {spec.valid
                  ? "Configured from pasted croncompose.yml"
                  : "Pasted croncompose.yml has errors and was not applied"}
              </strong>
              <span className="subtle"> · you can still change anything below.</span>
            </div>
            {spec.issues.length > 0 && <IssueList issues={spec.issues} />}
          </div>
        )}
        {!spec && inspect && (
          <div className="spec-banner">
            <div>
              Detected <strong>{inspect.language}</strong>
              {inspect.install_script && <> · <code>{inspect.install_script}</code></>}.{" "}
              <span className="subtle">
                Want this in code? <Link href="/docs">Add a croncompose.yml</Link> or export one below.
              </span>
            </div>
          </div>
        )}

        <DeployConfigTabs
          projectName={form.name}
          onProjectName={(name) => setForm((f) => ({ ...f, name }))}
          branch={form.branch}
          onBranch={(branch) => setForm((f) => ({ ...f, branch }))}
          branches={branches}
          provider={source.provider}
          repo={source.fullName}
          showBranch={source.provider === "github" || source.provider === "gitlab"}
          showSpecPicker={showSpecPicker}
          specFileOptions={specFileOptions}
          selectedSpecPath={selectedSpecPath}
          onSelectSpecPath={(path) => void onSelectSpecPath(path)}
          specBusy={busy === "spec"}
          blocks={form.blocks}
          onBlocks={(blocks) => setForm((f) => ({ ...f, blocks }))}
          apps={apps}
          globalEnv={form.globalEnv}
          onGlobalEnv={(globalEnv) => setForm((f) => ({ ...f, globalEnv }))}
          appEnv={form.appEnv}
          onAppEnv={(appEnv) => setForm((f) => ({ ...f, appEnv }))}
          advanced={form.advanced}
          onAdvanced={(advanced) => setForm((f) => ({ ...f, advanced }))}
          clonePath={form.clonePath}
          onClonePath={(clonePath) => setForm((f) => ({ ...f, clonePath }))}
          inspect={inspect}
          redeployOn={form.redeployOn}
          onRedeployOn={(redeployOn) => setForm((f) => ({ ...f, redeployOn }))}
          gitConnected={gitConnected}
        />

        {showYaml && (
          <div className="panel config-card spec-export" style={{ marginTop: 16 }}>
            <div className="row">
              <div className="card-title">croncompose.yml</div>
              <div className="cluster">
                <CopyButton value={exportYaml} className="light" />
                <a
                  className="button secondary sm"
                  download="croncompose.yml"
                  href={`data:text/yaml;charset=utf-8,${encodeURIComponent(exportYaml)}`}
                >
                  <IconDownload /> Download
                </a>
              </div>
            </div>
            <p className="field-hint">Commit this to the repo root and the next import fills itself in.</p>
            <pre className="spec-code">{exportYaml}</pre>
          </div>
        )}

        {error && <p className="form-error">{error}</p>}

        <div className="deploy-bar">
          <button type="button" className="button ghost sm" onClick={() => setShowYaml((v) => !v)}>
            {showYaml ? "Hide" : "Export as"} croncompose.yml
          </button>
          <div className="deploy-bar-right">
            {problems.length > 0 && <span className="subtle deploy-bar-hint">{problems[0]}</span>}
            <button
              type="button"
              className="button deploy-button"
              disabled={!canContinue}
              onClick={() => {
                setError(null);
                setPhase("review");
                window.scrollTo({ top: 0 });
              }}
            >
              {busy === "spec" ? "Loading…" : "Next"}
            </button>
          </div>
        </div>
      </div>
    );
  }

  // ---- Import --------------------------------------------------------------

  const providerTabs = (conns || []).map((c) => c.provider);
  return (
    <div className="deploy-flow wide">
      <button
        type="button"
        className="back-link as-button"
        onClick={() => setPhase("server")}
      >
        <IconChevronLeft /> Back to servers
      </button>
      <div className="page-head">
        <div>
          <h1>New project</h1>
          <p className="subtle">Pick a repo. CronCompose detects how to build it, and you deploy in one click.</p>
        </div>
      </div>

      <DeployServerChip server={selectedServer} onChange={() => setPhase("server")} />

      {error && <p className="form-error">{error}</p>}

      <div className="import-grid">
        <section className="panel import-card">
          <div className="row">
            <div className="card-title">Import Git repository</div>
            {providerTabs.length > 1 && (
              <div className="seg" role="tablist" aria-label="Git provider">
                {providerTabs.map((p) => (
                  <button
                    key={p}
                    type="button"
                    role="tab"
                    aria-selected={p === provider}
                    className={p === provider ? "on" : ""}
                    onClick={() => setProvider(p)}
                  >
                    {p === "github" ? "GitHub" : "GitLab"}
                  </button>
                ))}
              </div>
            )}
          </div>

          {conns === null && <p className="subtle">Loading…</p>}
          {conns !== null && conns.length === 0 && (
            <div className="connect-empty">
              <p className="subtle">Connect GitHub or GitLab to list your repos and deploy on every push.</p>
              <GitConnections initial={[]} next="/app/deploys/new" />
            </div>
          )}
          {connected && (
            <>
              <div className="search" style={{ margin: "14px 0 10px" }}>
                <IconSearch />
                <input
                  type="search"
                  value={repoQuery}
                  onChange={(e) => setRepoQuery(e.target.value)}
                  placeholder="Search…"
                  aria-label="Search repositories"
                  autoFocus
                />
              </div>
              {owners.length > 1 && (
                <div className="chips" role="group" aria-label="Filter by owner" style={{ marginBottom: 10 }}>
                  <button
                    type="button"
                    className={`chip${ownerFilter.length === 0 ? " selected" : ""}`}
                    aria-pressed={ownerFilter.length === 0}
                    onClick={() => setOwnerFilter([])}
                  >
                    All
                  </button>
                  {owners.map((o) => {
                    const on = ownerFilter.includes(o.owner);
                    return (
                      <button
                        key={o.owner}
                        type="button"
                        className={`chip${on ? " selected" : ""}`}
                        aria-pressed={on}
                        onClick={() => setOwnerFilter((prev) => toggleOwnerFilter(prev, o.owner))}
                      >
                        {o.personal ? `Personal · ${o.owner}` : o.owner}
                      </button>
                    );
                  })}
                </div>
              )}
              <ul className="repo-list">
                {repos === null && <li className="repo-empty subtle">Loading repositories…</li>}
                {repos?.length === 0 && <li className="repo-empty subtle">No repositories visible to this connection.</li>}
                {repos && repos.length > 0 && visibleRepos.length === 0 && (
                  <li className="repo-empty subtle">No repositories match “{repoQuery}”.</li>
                )}
                {visibleRepos.map((r) => {
                  const langIcon = githubLanguageIconUrl(r.language);
                  return (
                  <li key={r.id} className="repo-row">
                    <span className="mini-icon">
                      {langIcon ? (
                        // eslint-disable-next-line @next/next/no-img-element
                        <img src={langIcon} alt="" width={16} height={16} />
                      ) : (
                        <IconGit />
                      )}
                    </span>
                    <div className="repo-meta">
                      <div className="repo-name">
                        {r.full_name}
                        {r.private && <span className="pill">private</span>}
                      </div>
                      {r.description && <div className="repo-desc">{r.description}</div>}
                    </div>
                    <button
                      type="button"
                      className="button sm"
                      disabled={!!busy}
                      onClick={() => importRepo(r)}
                    >
                      {busy === `repo:${r.id}` ? "Reading…" : "Import"}
                    </button>
                  </li>
                  );
                })}
              </ul>
            </>
          )}
          {conns !== null && conns.length > 0 && !connected && (
            <p className="subtle">Connect this provider in <Link href="/settings">Settings</Link>.</p>
          )}
        </section>

        <div className="import-side">
          <section className="panel import-card">
            <div className="card-title">Clone a public repository</div>
            <p className="subtle small">No connection needed. Paste the URL.</p>
            <form
              className="inline-form"
              onSubmit={(e) => {
                e.preventDefault();
                importPublic();
              }}
            >
              <input
                value={publicUrl}
                onChange={(e) => setPublicUrl(e.target.value)}
                placeholder="https://github.com/owner/repo"
                aria-label="Public repository URL"
              />
              <button type="submit" className="button sm" disabled={!!busy || !publicUrl.trim()}>
                {busy === "public" ? "Reading…" : "Continue"}
              </button>
            </form>
          </section>

          <section className="panel import-card">
            <div className="row">
              <div className="card-title">Deploy from croncompose.yml</div>
              <Link href="/docs" className="small">Docs →</Link>
            </div>
            <p className="subtle small">
              Paste or upload a file. Already in your repo? Just import the repo; it is picked up automatically.
            </p>
            <textarea
              className="spec-input"
              value={yamlText}
              onChange={(e) => {
                setYamlText(e.target.value);
                setYamlIssues([]);
              }}
              placeholder={"version: 1\nrepo: owner/name\ninstall: npm ci && npm run build\nprocess_manager: pm2"}
              spellCheck={false}
              aria-label="croncompose.yml contents"
              rows={8}
            />
            <input
              ref={fileRef}
              type="file"
              accept=".yml,.yaml,text/yaml,application/x-yaml"
              hidden
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) void loadFile(f);
                e.target.value = "";
              }}
            />
            {yamlIssues.length > 0 && <IssueList issues={yamlIssues} />}
            <div className="cluster" style={{ marginTop: 10 }}>
              <button type="button" className="button sm" disabled={!!busy || !yamlText.trim()} onClick={importYaml}>
                {busy === "yaml" ? "Checking…" : "Continue"}
              </button>
              <button type="button" className="button secondary sm" onClick={() => fileRef.current?.click()}>
                Upload file
              </button>
              {!yamlText && (
                <button type="button" className="button ghost sm" onClick={() => setYamlText(SPEC_TEMPLATE)}>
                  Start from template
                </button>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

function IssueList({ issues }: { issues: DeploySpecIssue[] }) {
  return (
    <ul className="issue-list">
      {issues.map((is, i) => (
        <li key={i} className={`issue ${is.level}`}>
          {is.field && !is.message.includes(is.field) && <code>{is.field}</code>} {is.message}
        </li>
      ))}
    </ul>
  );
}
