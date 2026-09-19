"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { GitDirEntry, GitDirList } from "@/lib/types";
import { normalizeBlockRoot } from "@/lib/project-blocks";
import { apiErrorMessage } from "@/lib/api-error";

type Props = {
  provider: string;
  repo: string;
  branch: string;
  value: string;
  workspaces: string[];
  onChange: (root: string) => void;
  onClose: () => void;
};

function parentPath(path: string): string {
  const parts = path.split("/").filter(Boolean);
  parts.pop();
  return parts.join("/");
}

export function RepoFolderPicker({
  provider,
  repo,
  branch,
  value,
  workspaces,
  onChange,
  onClose,
}: Props) {
  const initial = value && value !== "." ? normalizeBlockRoot(value) : "";
  const browseStart = initial === "." ? "" : initial;
  const [browsePath, setBrowsePath] = useState(browseStart === "." ? "" : browseStart);
  const [items, setItems] = useState<GitDirEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [treeMode, setTreeMode] = useState(false);
  const [filter, setFilter] = useState("");
  const [manual, setManual] = useState(browseStart === "." ? "" : browseStart);

  const load = useCallback(
    async (path: string, recursive: boolean) => {
      setLoading(true);
      setError(null);
      try {
        const normalized = normalizeBlockRoot(path);
        const q = new URLSearchParams({
          provider,
          repo,
          branch,
          path: normalized === "." ? "" : normalized,
        });
        if (recursive) q.set("recursive", "1");
        const res = await fetch(`/api/git/dirs?${q}`);
        if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not list folders"));
        const data = (await res.json()) as GitDirList;
        setItems(data.items ?? []);
      } catch (e) {
        setError((e as Error).message);
        setItems([]);
      } finally {
        setLoading(false);
      }
    },
    [provider, repo, branch],
  );

  useEffect(() => {
    void load(treeMode ? "" : browsePath, treeMode);
  }, [load, browsePath, treeMode]);

  const visible = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return items;
    return items.filter(
      (item) =>
        item.path.toLowerCase().includes(q) || item.name.toLowerCase().includes(q),
    );
  }, [items, filter]);

  const currentRoot = browsePath === "" ? "." : browsePath;
  const pathLabel = currentRoot === "." ? "Repository root" : currentRoot;

  function commit(root: string) {
    onChange(normalizeBlockRoot(root));
    onClose();
  }

  return (
    <div
      className="panel"
      style={{ marginTop: 10, padding: 14, border: "1px solid var(--border-strong)" }}
      role="dialog"
      aria-label="Choose root folder"
    >
      <div className="row" style={{ justifyContent: "space-between", marginBottom: 10 }}>
        <div style={{ fontWeight: 700, fontSize: 14 }}>Choose root folder</div>
        <button type="button" className="button ghost sm" onClick={onClose}>
          Cancel
        </button>
      </div>

      {workspaces.length > 0 && (
        <div className="chips" style={{ marginBottom: 12 }}>
          {workspaces.map((w) => (
            <button
              key={w}
              type="button"
              className={`chip${normalizeBlockRoot(value) === normalizeBlockRoot(w) ? " selected" : ""}`}
              onClick={() => commit(w)}
            >
              {w}
            </button>
          ))}
        </div>
      )}

      <div className="row" style={{ gap: 8, marginBottom: 10, flexWrap: "wrap", alignItems: "center" }}>
        <code style={{ fontSize: 13 }}>{pathLabel}</code>
        <button
          type="button"
          className="button sm"
          onClick={() => commit(currentRoot)}
          disabled={loading}
        >
          Use this folder
        </button>
        {!treeMode && browsePath !== "" && (
          <button type="button" className="button secondary sm" onClick={() => setBrowsePath(parentPath(browsePath))}>
            Up
          </button>
        )}
        <button
          type="button"
          className="button ghost sm"
          onClick={() => {
            setTreeMode((t) => !t);
            setFilter("");
          }}
        >
          {treeMode ? "Browse folders" : "Search all folders"}
        </button>
      </div>

      {(treeMode || items.length > 12) && (
        <input
          type="search"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder={treeMode ? "Filter folders…" : "Filter…"}
          aria-label="Filter folders"
          style={{ marginBottom: 10 }}
        />
      )}

      <div className="stack" style={{ maxHeight: 220, overflow: "auto", gap: 4 }}>
        {loading && <p className="subtle" style={{ margin: 0 }}>Loading…</p>}
        {!loading && !treeMode && items.length === 0 && !error && (
          <p className="subtle" style={{ margin: 0 }}>No subfolders here — use this folder, or type a path below.</p>
        )}
        {!loading &&
          visible.map((item) => (
            <button
              key={item.path}
              type="button"
              className="button secondary sm"
              style={{ textAlign: "left", justifyContent: "flex-start", width: "100%" }}
              onClick={() => {
                if (treeMode) commit(item.path);
                else setBrowsePath(item.path);
              }}
            >
              <code>{treeMode ? item.path : `${item.name}/`}</code>
            </button>
          ))}
        {!loading && treeMode && visible.length === 0 && !error && (
          <p className="subtle" style={{ margin: 0 }}>No folders match.</p>
        )}
      </div>

      <div className="row" style={{ gap: 8, marginTop: 12 }}>
        <input
          value={manual}
          onChange={(e) => setManual(e.target.value)}
          placeholder="Or type a path (e.g. backend)"
          aria-label="Folder path"
          onKeyDown={(e) => {
            if (e.key === "Enter" && manual.trim()) commit(manual.trim());
          }}
        />
        <button
          type="button"
          className="button secondary sm"
          disabled={!manual.trim()}
          onClick={() => commit(manual.trim())}
        >
          Apply
        </button>
      </div>

      {error && <p className="form-error" style={{ marginTop: 10 }}>{error}</p>}
    </div>
  );
}
