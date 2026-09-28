"use client";

import { useEffect, useMemo, useState } from "react";
import type { GitDirEntry, GitDirList } from "@/lib/types";
import { normalizeBlockRoot } from "@/lib/project-blocks";
import { apiErrorMessage } from "@/lib/api-error";
import { SearchableSelect } from "@/components/SearchableSelect";
import type { SelectOption } from "@/lib/ui-helpers";

type Props = {
  provider: string;
  repo: string;
  branch: string;
  value: string;
  workspaces: string[];
  onChange: (root: string) => void;
  onClose: () => void;
};

async function fetchDirs(
  provider: string,
  repo: string,
  branch: string,
  path: string,
  recursive: boolean,
): Promise<GitDirEntry[]> {
  const q = new URLSearchParams({ provider, repo, branch, path });
  if (recursive) q.set("recursive", "1");
  const res = await fetch(`/api/git/dirs?${q}`);
  if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not list folders"));
  const data = (await res.json()) as GitDirList;
  return data.items ?? [];
}

function toOptions(items: GitDirEntry[]): SelectOption[] {
  return items.map((i) => ({ value: i.name, label: i.name }));
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
  // The path this picker opened on, split into one dropdown per level: editing
  // ["backend", "cmd"] shows a root-level dropdown (picks "backend"), a dropdown
  // of backend/'s children (picks "cmd"), and one more for cmd/'s children, if any.
  const initialSegments = useMemo(() => {
    const n = normalizeBlockRoot(value);
    return n === "." ? [] : n.split("/").filter(Boolean);
    // Only used to seed state when the picker opens; not meant to re-run on every
    // keystroke elsewhere, so this intentionally ignores `value` after mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [segments, setSegments] = useState<string[]>(initialSegments);
  const [levels, setLevels] = useState<GitDirEntry[][]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [treeMode, setTreeMode] = useState(false);
  const [treeItems, setTreeItems] = useState<GitDirEntry[]>([]);
  const [filter, setFilter] = useState("");

  const [manual, setManual] = useState("");

  // Load every level from the repo root down to the folder this picker opened on,
  // so re-opening it on an already-chosen deep folder shows the full breadcrumb of
  // dropdowns rather than just the root one.
  useEffect(() => {
    let live = true;
    setLoading(true);
    setError(null);
    (async () => {
      try {
        const built: GitDirEntry[][] = [];
        let path = "";
        for (let d = 0; d <= initialSegments.length; d++) {
          const items = await fetchDirs(provider, repo, branch, path, false);
          if (!live) return;
          built.push(items);
          if (d < initialSegments.length) {
            path = path ? `${path}/${initialSegments[d]}` : initialSegments[d];
          }
        }
        if (live) setLevels(built);
      } catch (e) {
        if (live) setError((e as Error).message);
      } finally {
        if (live) setLoading(false);
      }
    })();
    return () => {
      live = false;
    };
  }, [provider, repo, branch, initialSegments]);

  /** Picks `name` at `depth`, dropping any deeper levels, and loads its children
   * as the next dropdown (empty when it's a leaf folder). */
  async function selectAt(depth: number, name: string) {
    const nextSegments = [...segments.slice(0, depth), name];
    setSegments(nextSegments);
    setLevels((prev) => prev.slice(0, depth + 1));
    setLoading(true);
    setError(null);
    try {
      const items = await fetchDirs(provider, repo, branch, nextSegments.join("/"), false);
      setLevels((prev) => [...prev.slice(0, depth + 1), items]);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  function clearFrom(depth: number) {
    setSegments((prev) => prev.slice(0, depth));
    setLevels((prev) => prev.slice(0, depth + 1));
  }

  async function toggleTreeMode() {
    const next = !treeMode;
    setTreeMode(next);
    setFilter("");
    if (next && treeItems.length === 0) {
      setLoading(true);
      setError(null);
      try {
        setTreeItems(await fetchDirs(provider, repo, branch, "", true));
      } catch (e) {
        setError((e as Error).message);
      } finally {
        setLoading(false);
      }
    }
  }

  const filteredTree = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return treeItems;
    return treeItems.filter((i) => i.path.toLowerCase().includes(q) || i.name.toLowerCase().includes(q));
  }, [treeItems, filter]);

  const currentRoot = segments.length ? segments.join("/") : ".";
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
        <button type="button" className="button sm" onClick={() => commit(currentRoot)} disabled={loading}>
          Use this folder
        </button>
        <button type="button" className="button ghost sm" onClick={() => void toggleTreeMode()}>
          {treeMode ? "Back to folder-by-folder" : "Search all folders"}
        </button>
      </div>

      {!treeMode && (
        <div className="stack" style={{ gap: 8 }}>
          {levels.map((items, depth) => {
            if (depth > 0 && items.length === 0 && depth <= segments.length) return null;
            const atLeaf = depth === segments.length && items.length === 0;
            if (atLeaf && depth > 0) return null;
            return (
              <div key={depth} className="row" style={{ gap: 8, alignItems: "center" }}>
                <span className="subtle" style={{ fontSize: 12, minWidth: 64 }}>
                  {depth === 0 ? "Root" : segments.slice(0, depth).join("/")}
                </span>
                <div style={{ flex: 1, minWidth: 160 }}>
                  <SearchableSelect
                    value={segments[depth] ?? ""}
                    options={toOptions(items)}
                    placeholder={items.length ? "Choose a subfolder…" : "No subfolders here"}
                    disabled={loading || items.length === 0}
                    onChange={(name) => void selectAt(depth, name)}
                    aria-label={`Subfolder at level ${depth + 1}`}
                  />
                </div>
                {depth < segments.length && (
                  <button type="button" className="button ghost sm" onClick={() => clearFrom(depth)}>
                    Clear
                  </button>
                )}
              </div>
            );
          })}
          {loading && <p className="subtle" style={{ margin: 0 }}>Loading…</p>}
        </div>
      )}

      {treeMode && (
        <>
          <input
            type="search"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter folders…"
            aria-label="Filter folders"
            style={{ marginBottom: 10 }}
          />
          <div className="stack" style={{ maxHeight: 220, overflow: "auto", gap: 4 }}>
            {loading && <p className="subtle" style={{ margin: 0 }}>Loading…</p>}
            {!loading &&
              filteredTree.map((item) => (
                <button
                  key={item.path}
                  type="button"
                  className="button secondary sm"
                  style={{ textAlign: "left", justifyContent: "flex-start", width: "100%" }}
                  onClick={() => commit(item.path)}
                >
                  <code>{item.path}</code>
                </button>
              ))}
            {!loading && filteredTree.length === 0 && !error && (
              <p className="subtle" style={{ margin: 0 }}>No folders match.</p>
            )}
          </div>
        </>
      )}

      <div className="row" style={{ gap: 8, marginTop: 12 }}>
        <input
          value={manual}
          onChange={(e) => setManual(e.target.value)}
          placeholder="Or type a path (e.g. backend/cmd)"
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
