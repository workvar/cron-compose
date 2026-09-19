"use client";

import { useState } from "react";
import {
  nameFromRoot,
  normalizeBlockRoot,
  type ProjectBlock,
} from "@/lib/project-blocks";
import { RepoFolderPicker } from "./RepoFolderPicker";

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

  function patch(partial: Partial<ProjectBlock>) {
    onChange({ ...block, ...partial });
  }

  function onRootChange(root: string) {
    const next: ProjectBlock = { ...block, root };
    if (!block.name.trim() || block.name === nameFromRoot(block.root, repo)) {
      next.name = nameFromRoot(root, repo);
    }
    onChange(next);
  }

  const rootDisplay = block.root.trim()
    ? normalizeBlockRoot(block.root)
    : "(pick a folder)";

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
          onChange={onRootChange}
          onClose={() => setPickerOpen(false)}
        />
      )}

      <div className="grid-2" style={{ marginTop: 12 }}>
        <div className="field">
          <label htmlFor={`block-lang-${block.id}`}>Language</label>
          <input
            id={`block-lang-${block.id}`}
            value={block.language}
            onChange={(e) => patch({ language: e.target.value })}
          />
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
        <label htmlFor={`block-install-${block.id}`}>Install script</label>
        <textarea
          id={`block-install-${block.id}`}
          rows={3}
          value={block.install}
          onChange={(e) => patch({ install: e.target.value })}
        />
      </div>

      <div className="field">
        <label htmlFor={`block-pm-${block.id}`}>Process manager</label>
        <select
          id={`block-pm-${block.id}`}
          value={block.processManager}
          onChange={(e) => patch({ processManager: e.target.value })}
        >
          <option value="none">None — attach later</option>
          <option value="pm2">PM2</option>
          <option value="systemd">systemd (user unit)</option>
          <option value="docker">Docker Compose</option>
        </select>
      </div>
    </div>
  );
}
