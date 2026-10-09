"use client";

import { useState, type ReactNode } from "react";
import {
  applyDetection,
  applyFramework,
  emptyBlock,
  nameFromRoot,
  normalizeBlockRoot,
  type ProjectBlock,
  type RedeployMode,
} from "@/lib/project-blocks";
import type { AdvancedSettings } from "@/lib/deploy-spec";
import { detectAt } from "@/lib/git-detect";
import { languageSelectOptions } from "@/lib/language-icons";
import { PROCESS_MANAGER_OPTIONS } from "@/lib/process-managers";
import type { DeployApp, DeployEnvVar, DeployInspect } from "@/lib/types";
import type { SelectOption } from "@/lib/ui-helpers";
import { SearchableSelect } from "@/components/SearchableSelect";
import { AppEnvEditor } from "./AppEnvEditor";
import { HealthCheckFields } from "./HealthCheckFields";
import { RepoFolderPicker } from "./RepoFolderPicker";
import { GitConnections } from "./GitConnections";
import { UserSwitcher } from "@/components/terminal/UserSwitcher";

export type ConfigTabId =
  | "processes"
  | "build"
  | "env"
  | "process-manager"
  | "advanced"
  | "redeploy";

type Props = {
  projectName: string;
  onProjectName: (name: string) => void;
  branch: string;
  onBranch: (branch: string) => void;
  branches: SelectOption[];
  provider: string;
  repo: string;
  showBranch: boolean;
  showSpecPicker: boolean;
  specFileOptions: SelectOption[];
  selectedSpecPath: string;
  onSelectSpecPath: (path: string) => void;
  specBusy: boolean;
  blocks: ProjectBlock[];
  onBlocks: (blocks: ProjectBlock[]) => void;
  apps: DeployApp[];
  globalEnv: DeployEnvVar[];
  onGlobalEnv: (env: DeployEnvVar[]) => void;
  appEnv: Record<string, DeployEnvVar[]>;
  onAppEnv: (appEnv: Record<string, DeployEnvVar[]>) => void;
  advanced: AdvancedSettings;
  onAdvanced: (advanced: AdvancedSettings) => void;
  clonePath: string;
  onClonePath: (path: string) => void;
  /** Target server for the deploy-as user picker. */
  serverId: string;
  runAsUser: string;
  onRunAsUser: (user: string) => void;
  inspect: DeployInspect | null;
  redeployOn: RedeployMode[];
  onRedeployOn: (modes: RedeployMode[]) => void;
  gitConnected: boolean;
  activeTab?: ConfigTabId;
  onTabChange?: (tab: ConfigTabId) => void;
};

const TABS: { id: ConfigTabId; label: string }[] = [
  { id: "processes", label: "Processes" },
  { id: "build", label: "Build & run" },
  { id: "env", label: "Environment" },
  { id: "process-manager", label: "Process manager" },
  { id: "advanced", label: "Advanced" },
  { id: "redeploy", label: "Redeploy" },
];

function toggleMode(modes: RedeployMode[], mode: RedeployMode): RedeployMode[] {
  if (modes.includes(mode)) {
    const next = modes.filter((m) => m !== mode);
    return next.length ? next : ["branch"];
  }
  return [...modes, mode];
}

export function DeployConfigTabs(props: Props) {
  const [tab, setTab] = useState<ConfigTabId>(props.activeTab || "processes");
  const active = props.activeTab ?? tab;
  function selectTab(id: ConfigTabId) {
    setTab(id);
    props.onTabChange?.(id);
  }

  const panels: Record<ConfigTabId, ReactNode> = {
    processes: <ProcessesTab {...props} />,
    build: <BuildRunTab {...props} />,
    env: (
      <div className="config-tab-panel">
        <p className="field-hint" style={{ marginTop: 0 }}>
          Shared variables apply to every process. Process-level values override the same key.
          Mark secrets as sensitive on a process so they stay encrypted and never go into{" "}
          <code>croncompose.yml</code>.
        </p>
        <AppEnvEditor
          apps={props.apps}
          title=""
          globalEnv={props.globalEnv}
          onGlobalEnv={props.onGlobalEnv}
          onChange={(next) => {
            const appEnv: Record<string, DeployEnvVar[]> = {};
            for (const a of next) appEnv[a.name] = a.env ?? [];
            props.onAppEnv(appEnv);
          }}
        />
      </div>
    ),
    "process-manager": <ProcessManagerTab {...props} />,
    advanced: <AdvancedTab {...props} />,
    redeploy: <RedeployTab {...props} />,
  };

  return (
    <div className="deploy-config-tabs">
      <div className="panel config-card" style={{ marginBottom: 16 }}>
        <div className="grid-2">
          <div className="field">
            <label htmlFor="name">Project name</label>
            <input
              id="name"
              value={props.projectName}
              onChange={(e) => props.onProjectName(e.target.value)}
            />
          </div>
          {props.showBranch && (
            <div className="field">
              <label htmlFor="branch">Default branch</label>
              <SearchableSelect
                id="branch"
                value={props.branch}
                options={props.branches}
                allowCustom
                placeholder="main"
                onChange={props.onBranch}
                aria-label="Branch"
              />
              <p className="field-hint">Used for the first deploy and for branch-push redeploys.</p>
            </div>
          )}
        </div>
        {props.showSpecPicker && (
          <div className="field" style={{ marginTop: 12 }}>
            <label htmlFor="specFile">croncompose.yml</label>
            <SearchableSelect
              id="specFile"
              value={props.selectedSpecPath}
              options={props.specFileOptions}
              onChange={props.onSelectSpecPath}
              placeholder="Select a file…"
              aria-label="croncompose.yml from repo"
              disabled={props.specBusy}
            />
            <p className="field-hint">
              Pick which file in the repo drives this deploy, or None to detect from the tree.
            </p>
          </div>
        )}
      </div>

      <div className="cc-tabs deploy-config-tablist" role="tablist" aria-label="Deployment options">
        {TABS.map((t) => {
          const selected = t.id === active;
          return (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={selected}
              className={`cc-tab${selected ? " active" : ""}`}
              onClick={() => selectTab(t.id)}
            >
              {t.label}
            </button>
          );
        })}
      </div>
      <div className="panel config-card" role="tabpanel">
        {panels[active]}
      </div>
    </div>
  );
}

function ProcessesTab({
  blocks,
  onBlocks,
  inspect,
  provider,
  repo,
  branch,
}: Props) {
  return (
    <div className="config-tab-panel">
      <p className="field-hint" style={{ marginTop: 0 }}>
        Select one or more processes (apps) from this repository. Each gets its own folder, build,
        and run settings. In a monorepo, add an app per package you want to host.
      </p>
      <div className="stack">
        {blocks.map((block) => (
          <ProcessCard
            key={block.id}
            block={block}
            workspaces={inspect?.workspaces || []}
            provider={provider}
            repo={repo}
            branch={branch}
            canRemove={blocks.length > 1}
            onChange={(next) => onBlocks(blocks.map((b) => (b.id === next.id ? next : b)))}
            onRemove={() => onBlocks(blocks.filter((b) => b.id !== block.id))}
          />
        ))}
      </div>
      <button
        type="button"
        className="button ghost sm"
        style={{ marginTop: 12 }}
        onClick={() => onBlocks([...blocks, emptyBlock()])}
      >
        + Add another process
      </button>
    </div>
  );
}

function ProcessCard({
  block,
  workspaces,
  provider,
  repo,
  branch,
  canRemove,
  onChange,
  onRemove,
}: {
  block: ProjectBlock;
  workspaces: string[];
  provider: string;
  repo: string;
  branch: string;
  canRemove: boolean;
  onChange: (block: ProjectBlock) => void;
  onRemove: () => void;
}) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const [detecting, setDetecting] = useState(false);

  function patch(partial: Partial<ProjectBlock>) {
    onChange({ ...block, ...partial });
  }

  async function onRootChange(root: string) {
    const next: ProjectBlock = { ...block, root };
    if (!block.name.trim() || block.name === nameFromRoot(block.root, repo)) {
      next.name = nameFromRoot(root, repo);
    }
    onChange(next);
    if (!next.autoDetect) return;
    setDetecting(true);
    try {
      const det = await detectAt(provider, repo, branch, root);
      onChange(applyDetection(next, det));
    } catch {
      // Detection is a convenience; leave previous values on failure.
    } finally {
      setDetecting(false);
    }
  }

  const rootDisplay = block.root.trim()
    ? normalizeBlockRoot(block.root)
    : "(pick a folder)";
  const languageOptions = languageSelectOptions(block.language);

  return (
    <div className="process-card">
      <div className="row" style={{ justifyContent: "space-between", alignItems: "flex-start" }}>
        <div className="card-title">Process</div>
        {canRemove && (
          <button type="button" className="button ghost sm" onClick={onRemove}>
            Remove
          </button>
        )}
      </div>
      <div className="grid-2" style={{ marginTop: 12 }}>
        <div className="field">
          <label htmlFor={`block-name-${block.id}`}>Name</label>
          <input
            id={`block-name-${block.id}`}
            value={block.name}
            onChange={(e) => patch({ name: e.target.value })}
          />
          <p className="field-hint">
            Deploy folder uses <code>project-name</code> / <code>{block.name || "process"}</code> under the
            server path (usually <code>/opt/…</code>).
          </p>
        </div>
        <div className="field">
          <label>Root folder in repo</label>
          <div className="row" style={{ gap: 8 }}>
            <code style={{ flex: 1, padding: "8px 10px", background: "var(--surface-2)", borderRadius: 8 }}>
              {rootDisplay}
            </code>
            <button
              type="button"
              className="button secondary sm"
              onClick={() => setPickerOpen((o) => !o)}
            >
              Change
            </button>
          </div>
        </div>
      </div>
      {pickerOpen && (
        <RepoFolderPicker
          provider={provider}
          repo={repo}
          branch={branch}
          value={block.root || "."}
          workspaces={workspaces}
          onChange={(root) => {
            void onRootChange(root);
          }}
          onClose={() => setPickerOpen(false)}
        />
      )}
      <div className="grid-2" style={{ marginTop: 12 }}>
        <div className="field">
          <label htmlFor={`block-lang-${block.id}`}>Framework / language</label>
          <SearchableSelect
            id={`block-lang-${block.id}`}
            value={block.language}
            options={languageOptions}
            allowCustom
            placeholder={detecting ? "Detecting…" : "Select a framework…"}
            disabled={detecting}
            onChange={(language) => onChange(applyFramework(block, language))}
            aria-label="Framework or language"
          />
          <p className="field-hint">
            Fills build script, run command, port, and process manager for the selected stack.
          </p>
        </div>
        <div className="field">
          <label htmlFor={`block-port-${block.id}`}>PORT (optional)</label>
          <input
            id={`block-port-${block.id}`}
            inputMode="numeric"
            value={block.port}
            onChange={(e) => patch({ port: e.target.value })}
            placeholder="auto-detect"
          />
          <p className="field-hint">
            Leave blank to let the agent detect the listening port and expose it in the web UI
            (Ports / Connectors).
          </p>
        </div>
      </div>
    </div>
  );
}

function BuildRunTab({ blocks, onBlocks, clonePath, projectName }: Props) {
  const deployHint =
    clonePath.trim() ||
    `/opt/apps/…/${(projectName || "project").trim() || "project"}`;

  return (
    <div className="config-tab-panel">
      <div className="build-run-callout">
        <strong>How build &amp; run work</strong>
        <p>
          CronCompose clones the repo into a temporary folder, then cherry-picks only each
          process&apos;s root directory into the release under <code>{deployHint}</code>. The{" "}
          <em>build script</em> runs inside that release folder; <em>cleanup</em> then drops
          source and caches. After activation, the <em>run script</em> starts the process
          (pm2 / systemd / docker) from that folder — for example <code>./app</code> or{" "}
          <code>npm start</code>. Process names are prefixed with the project name (e.g.{" "}
          <code>shop-web</code>) so apps from different projects do not collide. With pm2,
          CronCompose also runs <code>pm2 save</code> and best-effort <code>pm2 startup</code> so
          the app returns after reboot.
        </p>
      </div>
      <div className="stack">
        {blocks.map((block) => (
          <div key={block.id} className="process-card">
            <div className="card-title">{block.name || "Process"}</div>
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor={`block-build-${block.id}`}>Build script</label>
              <textarea
                id={`block-build-${block.id}`}
                rows={3}
                value={block.install}
                onChange={(e) =>
                  onBlocks(blocks.map((b) => (b.id === block.id ? { ...b, install: e.target.value } : b)))
                }
                placeholder="go build -o app ./cmd/server"
                spellCheck={false}
              />
              <p className="field-hint">
                Install deps, compile, migrate — anything that prepares artifacts before the process starts.
              </p>
            </div>
            <div className="field">
              <label htmlFor={`block-run-${block.id}`}>Run script</label>
              <input
                id={`block-run-${block.id}`}
                value={block.run}
                onChange={(e) =>
                  onBlocks(blocks.map((b) => (b.id === block.id ? { ...b, run: e.target.value } : b)))
                }
                placeholder="./app"
                spellCheck={false}
              />
              <p className="field-hint">
                Executed with cwd set to the activated deploy folder. Keep it short — the binary or
                start command only.
              </p>
            </div>
            <div className="field">
              <label htmlFor={`block-cleanup-${block.id}`}>Cleanup (after build)</label>
              <textarea
                id={`block-cleanup-${block.id}`}
                rows={2}
                value={block.cleanup}
                onChange={(e) =>
                  onBlocks(blocks.map((b) => (b.id === block.id ? { ...b, cleanup: e.target.value } : b)))
                }
                placeholder="rm -rf .git src node_modules/.cache"
                spellCheck={false}
              />
              <p className="field-hint">
                Removes source and other non-runtime files from the release after a successful build.
                Framework presets fill this automatically; leave blank to skip.
              </p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ProcessManagerTab({ blocks, onBlocks }: Props) {
  return (
    <div className="config-tab-panel">
      <p className="field-hint" style={{ marginTop: 0 }}>
        Choose what keeps each process running after the release is activated. The run script is
        handed to the manager as the start command when set.
      </p>
      <div className="stack">
        {blocks.map((block) => (
          <div key={block.id} className="process-card">
            <div className="card-title">{block.name || "Process"}</div>
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor={`block-pm-${block.id}`}>Process manager</label>
              <SearchableSelect
                id={`block-pm-${block.id}`}
                value={block.processManager}
                options={PROCESS_MANAGER_OPTIONS}
                onChange={(processManager) =>
                  onBlocks(
                    blocks.map((b) => (b.id === block.id ? { ...b, processManager } : b)),
                  )
                }
                aria-label="Process manager"
              />
            </div>
            <details className="advanced" open={!!block.healthPath} style={{ marginTop: 8 }}>
              <summary>Health check for this process</summary>
              <div style={{ marginTop: 14 }}>
                <HealthCheckFields
                  idPrefix={`block-${block.id}-`}
                  hideDeployTimeout
                  appPort={Number(block.port) || 0}
                  value={{
                    path: block.healthPath,
                    port: block.healthPort,
                    timeout: block.healthTimeout,
                    deployTimeout: "",
                  }}
                  onChange={(v) =>
                    onBlocks(
                      blocks.map((b) =>
                        b.id === block.id
                          ? {
                              ...b,
                              healthPath: v.path,
                              healthPort: v.port,
                              healthTimeout: v.timeout,
                            }
                          : b,
                      ),
                    )
                  }
                />
              </div>
            </details>
          </div>
        ))}
      </div>
    </div>
  );
}

function AdvancedTab({
  advanced,
  onAdvanced,
  clonePath,
  onClonePath,
  serverId,
  runAsUser,
  onRunAsUser,
  blocks,
  showBranch,
  branch,
  onBranch,
  branches,
}: Props) {
  return (
    <div className="config-tab-panel">
      {!showBranch && (
        <div className="field">
          <label htmlFor="branch-adv">Branch</label>
          <SearchableSelect
            id="branch-adv"
            value={branch}
            options={branches}
            allowCustom
            placeholder="main"
            onChange={onBranch}
            aria-label="Branch"
          />
        </div>
      )}
      {serverId ? (
        <div className="field">
          <label htmlFor="run-as-user">Deploy as</label>
          <UserSwitcher
            id="run-as-user"
            serverId={serverId}
            value={runAsUser}
            onChange={onRunAsUser}
          />
          <p className="field-hint">
            Clone, install, and start run as this OS account. Non-root accounts use{" "}
            <code>~/opt</code> and <code>~/tmp</code>, and see that user&apos;s login PATH
            (nvm, etc.).
          </p>
        </div>
      ) : null}
      <div className="field">
        <label htmlFor="clonePath">Folder on the server</label>
        <input
          id="clonePath"
          placeholder="/opt/apps/…"
          value={clonePath}
          onChange={(e) => onClonePath(e.target.value)}
        />
        <p className="field-hint">
          Releases land here. Each process runs from its app folder under{" "}
          <code>current/</code> after activation.
        </p>
      </div>
      {blocks.length > 1 && (
        <p className="field-hint">
          Shared health check below applies when a process does not set its own on the Process
          manager tab.
        </p>
      )}
      <HealthCheckFields
        idPrefix="project-"
        appPort={Number(blocks[0]?.port) || 0}
        value={{
          path: advanced.healthPath,
          port: advanced.healthPort,
          timeout: advanced.healthTimeout,
          deployTimeout: advanced.deployTimeout,
        }}
        onChange={(v) =>
          onAdvanced({
            ...advanced,
            healthPath: v.path,
            healthPort: v.port,
            healthTimeout: v.timeout,
            deployTimeout: v.deployTimeout,
          })
        }
      />
      <label className="check-row">
        <input
          type="checkbox"
          checked={advanced.autoRollback}
          onChange={(e) => onAdvanced({ ...advanced, autoRollback: e.target.checked })}
        />
        <span>
          <strong>Auto-rollback</strong>
          <span className="field-hint" style={{ display: "block", marginTop: 2 }}>
            If a deploy fails, go back to the last commit that worked.
          </span>
        </span>
      </label>
    </div>
  );
}

function RedeployTab({
  redeployOn,
  onRedeployOn,
  branch,
  onBranch,
  branches,
  provider,
  gitConnected,
}: Props) {
  const canGit = provider === "github" || provider === "gitlab";

  return (
    <div className="config-tab-panel">
      <p className="field-hint" style={{ marginTop: 0 }}>
        Connect {provider === "gitlab" ? "GitLab" : "GitHub"} so CronCompose can watch the repo and
        redeploy when matching events land. Pick one or more triggers below.
      </p>

      {!gitConnected && canGit && (
        <div className="connect-empty" style={{ marginBottom: 16 }}>
          <p className="subtle">No git connection for this provider yet.</p>
          <GitConnections initial={[]} next="/app/deploys/new" />
        </div>
      )}

      <div className="redeploy-modes" role="group" aria-label="Redeploy triggers">
        <label className={`redeploy-mode${redeployOn.includes("branch") ? " on" : ""}`}>
          <input
            type="checkbox"
            checked={redeployOn.includes("branch")}
            onChange={() => onRedeployOn(toggleMode(redeployOn, "branch"))}
          />
          <span>
            <strong>Branch push</strong>
            <span className="field-hint" style={{ display: "block", marginTop: 2 }}>
              Redeploy when code is pushed to the selected branch.
            </span>
          </span>
        </label>
        {redeployOn.includes("branch") && (
          <div className="field" style={{ marginLeft: 28, marginBottom: 12 }}>
            <label htmlFor="redeploy-branch">Branch</label>
            <SearchableSelect
              id="redeploy-branch"
              value={branch}
              options={branches}
              allowCustom
              placeholder="Search branches…"
              onChange={onBranch}
              aria-label="Redeploy branch"
            />
          </div>
        )}

        <label className={`redeploy-mode${redeployOn.includes("tag") ? " on" : ""}`}>
          <input
            type="checkbox"
            checked={redeployOn.includes("tag")}
            onChange={() => onRedeployOn(toggleMode(redeployOn, "tag"))}
          />
          <span>
            <strong>Tag push</strong>
            <span className="field-hint" style={{ display: "block", marginTop: 2 }}>
              Redeploy when a git tag is pushed (any tag).
            </span>
          </span>
        </label>

        <label className={`redeploy-mode${redeployOn.includes("release") ? " on" : ""}`}>
          <input
            type="checkbox"
            checked={redeployOn.includes("release")}
            onChange={() => onRedeployOn(toggleMode(redeployOn, "release"))}
          />
          <span>
            <strong>New release</strong>
            <span className="field-hint" style={{ display: "block", marginTop: 2 }}>
              Redeploy when a GitHub / GitLab release is published.
            </span>
          </span>
        </label>
      </div>

      <p className="field-hint" style={{ marginTop: 14 }}>
        After the first deploy, CronCompose installs a webhook (and a small CI trigger) on the
        connected repo. Manual redeploy from the project page can still target any branch, tag, or
        release.
      </p>
    </div>
  );
}
