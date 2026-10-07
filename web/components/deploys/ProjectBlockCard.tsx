"use client";

import { useState } from "react";
import {
  defaultRunForLanguage,
  nameFromRoot,
  normalizeBlockRoot,
  type ProjectBlock,
} from "@/lib/project-blocks";
import { detectAt } from "@/lib/git-detect";
import { languageSelectOptions } from "@/lib/language-icons";
import { PROCESS_MANAGER_OPTIONS } from "@/lib/process-managers";
import { SearchableSelect } from "@/components/SearchableSelect";
import { HealthCheckFields } from "./HealthCheckFields";
import { RepoFolderPicker } from "./RepoFolderPicker";
import { IconChevronRight } from "@/components/icons";

type Props = {
  block: ProjectBlock;
  workspaces: string[];
  provider: string;
  repo: string;
  branch: string;
  canRemove: boolean;
  onChange: (block: ProjectBlock) => void;
  onRemove: () => void;
};

export function ProjectBlockCard({
  block,
  workspaces,
  provider,
  repo,
  branch,
  canRemove,
  onChange,
  onRemove,
}: Props) {
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
    // Re-detect scoped to the new root, so a Go API under backend/ gets its own
    // language/install guess instead of inheriting the repo root's. Skipped once
    // the person has picked a framework by hand or this block came from an
    // explicit croncompose.yml (autoDetect is off in both cases).
    if (!next.autoDetect) return;
    setDetecting(true);
    try {
      const det = await detectAt(provider, repo, branch, root);
      onChange({
        ...next,
        language: det.language || "unknown",
        install: det.install_script || next.install,
        run: next.run || defaultRunForLanguage(det.language || next.language),
      });
    } catch {
      // Detection is a convenience; leave the previous values on failure.
    } finally {
      setDetecting(false);
    }
  }

  const rootDisplay = block.root.trim()
    ? normalizeBlockRoot(block.root)
    : "(pick a folder)";
  const languageOptions = languageSelectOptions(block.language);

  return (
    <div className="panel">
      <div className="row" style={{ justifyContent: "space-between", alignItems: "flex-start" }}>
        <div className="card-title">Project</div>
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
        </div>
        <div className="field">
          <label>Root folder</label>
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
            onChange={(language) => patch({ language, autoDetect: false })}
            aria-label="Framework or language"
          />
          <p className="field-hint">
            {detecting ? "Re-detecting from this folder…" : "Auto-detected from the repo; pick one to override it."}
          </p>
        </div>
        <div className="field">
          <label htmlFor={`block-port-${block.id}`}>PORT (optional)</label>
          <input
            id={`block-port-${block.id}`}
            inputMode="numeric"
            value={block.port}
            onChange={(e) => patch({ port: e.target.value })}
          />
        </div>
      </div>

      <div className="field" style={{ marginTop: 12 }}>
        <label htmlFor={`block-install-${block.id}`}>Build script</label>
        <textarea
          id={`block-install-${block.id}`}
          rows={3}
          value={block.install}
          onChange={(e) => patch({ install: e.target.value })}
        />
        <p className="field-hint">Runs after clone; artifacts stay in the release under the deploy path (usually /opt/…).</p>
      </div>

      <div className="field">
        <label htmlFor={`block-run-${block.id}`}>Run script</label>
        <input
          id={`block-run-${block.id}`}
          value={block.run}
          onChange={(e) => patch({ run: e.target.value })}
          placeholder="./app"
          spellCheck={false}
        />
        <p className="field-hint">Start command with cwd set to the activated app folder.</p>
      </div>

      <div className="field">
        <label htmlFor={`block-pm-${block.id}`}>Process manager</label>
        <SearchableSelect
          id={`block-pm-${block.id}`}
          value={block.processManager}
          options={PROCESS_MANAGER_OPTIONS}
          onChange={(processManager) => patch({ processManager })}
          aria-label="Process manager"
        />
      </div>

      <details className="advanced" open={!!block.healthPath} style={{ marginTop: 12 }}>
        <summary>
          <span className="chev"><IconChevronRight /></span> Health check
        </summary>
        <div style={{ marginTop: 14 }}>
          <p className="field-hint" style={{ marginTop: 0 }}>
            Overrides the project&apos;s shared health check for this app only. Leave empty to use the
            project-wide one (Advanced, below), if any.
          </p>
          <HealthCheckFields
            idPrefix={`block-${block.id}-`}
            hideDeployTimeout
            appPort={Number(block.port) || 0}
            value={{ path: block.healthPath, port: block.healthPort, timeout: block.healthTimeout, deployTimeout: "" }}
            onChange={(v) => patch({ healthPath: v.path, healthPort: v.port, healthTimeout: v.timeout })}
          />
        </div>
      </details>
    </div>
  );
}
