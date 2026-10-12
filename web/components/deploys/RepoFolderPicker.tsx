"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import type { GitDirEntry, GitDirList } from "@/lib/types";
import { normalizeBlockRoot } from "@/lib/project-blocks";
import { apiErrorMessage } from "@/lib/api-error";
import { IconChevronDown, IconChevronRight, IconFolder, IconFolderOpen } from "@/components/icons";

type Props = {
  provider: string;
  repo: string;
  branch: string;
  value: string;
  workspaces: string[];
  onChange: (root: string) => void;
  onClose: () => void;
};

const ROOT_KEY = "";

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

function pathKey(path: string): string {
  return normalizeBlockRoot(path) === "." ? ROOT_KEY : normalizeBlockRoot(path);
}

function parentKey(path: string): string {
  const n = pathKey(path);
  if (!n) return ROOT_KEY;
  const i = n.lastIndexOf("/");
  return i < 0 ? ROOT_KEY : n.slice(0, i);
}

function childEntries(all: GitDirEntry[], parent: string): GitDirEntry[] {
  const prefix = parent ? `${parent}/` : "";
  return all
    .filter((item) => {
      if (!prefix) return !item.path.includes("/");
      if (!item.path.startsWith(prefix)) return false;
      return !item.path.slice(prefix.length).includes("/");
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}

function ancestorsOf(path: string): string[] {
  const n = pathKey(path);
  if (!n) return [];
  const parts = n.split("/");
  const out: string[] = [];
  for (let i = 0; i < parts.length; i++) {
    out.push(parts.slice(0, i + 1).join("/"));
  }
  return out;
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
  const initial = useMemo(() => pathKey(value), [value]);

  const [selected, setSelected] = useState(initial || ".");
  const [childrenByPath, setChildrenByPath] = useState<Record<string, GitDirEntry[]>>({});
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [loadingPaths, setLoadingPaths] = useState<Set<string>>(() => new Set());
  const [error, setError] = useState<string | null>(null);

  const [filter, setFilter] = useState("");
  const [allDirs, setAllDirs] = useState<GitDirEntry[] | null>(null);
  const [loadingAll, setLoadingAll] = useState(false);

  const [manual, setManual] = useState("");
  const [showManual, setShowManual] = useState(false);

  async function loadChildren(path: string): Promise<GitDirEntry[]> {
    const key = pathKey(path);
    setLoadingPaths((prev) => new Set(prev).add(key));
    setError(null);
    try {
      const items = await fetchDirs(provider, repo, branch, key, false);
      setChildrenByPath((prev) => ({ ...prev, [key]: items }));
      return items;
    } catch (e) {
      setError((e as Error).message);
      return [];
    } finally {
      setLoadingPaths((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }

  // Seed root folders for this branch; clear caches so a branch switch cannot
  // show the previous tree.
  useEffect(() => {
    let live = true;
    setChildrenByPath({});
    setAllDirs(null);
    setExpanded(new Set());
    setFilter("");
    setError(null);
    (async () => {
      const rootItems = await loadChildren(ROOT_KEY);
      if (!live) return;
      if (!initial) return;

      const nextExpanded = new Set<string>();
      let parent = ROOT_KEY;
      for (const segment of initial.split("/")) {
        nextExpanded.add(parent);
        const items = parent === ROOT_KEY ? rootItems : await loadChildren(parent);
        if (!live) return;
        if (!items.some((i) => i.name === segment)) break;
        parent = parent ? `${parent}/${segment}` : segment;
      }
      nextExpanded.add(parentKey(initial));
      if (live) setExpanded(nextExpanded);
    })();
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [provider, repo, branch]);

  const filterQ = filter.trim().toLowerCase();
  const filtering = filterQ.length > 0;

  async function ensureAllDirs(): Promise<GitDirEntry[]> {
    if (allDirs) return allDirs;
    setLoadingAll(true);
    setError(null);
    try {
      const items = await fetchDirs(provider, repo, branch, "", true);
      setAllDirs(items);
      return items;
    } catch (e) {
      setError((e as Error).message);
      return [];
    } finally {
      setLoadingAll(false);
    }
  }

  async function toggleExpand(path: string) {
    if (filtering) return;
    const key = pathKey(path);
    const willExpand = !expanded.has(key);
    setExpanded((prev) => {
      const next = new Set(prev);
      if (willExpand) next.add(key);
      else next.delete(key);
      return next;
    });
    if (willExpand && childrenByPath[key] === undefined) {
      await loadChildren(key);
    }
  }

  useEffect(() => {
    if (!filtering) return;
    void ensureAllDirs();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filtering, provider, repo, branch]);

  const visibleChildren = useMemo(() => {
    if (!filtering || !allDirs) return childrenByPath;

    const matchPaths = new Set(
      allDirs
        .filter((i) => i.path.toLowerCase().includes(filterQ) || i.name.toLowerCase().includes(filterQ))
        .flatMap((i) => [i.path, ...ancestorsOf(i.path)]),
    );

    const keys = new Set<string>([ROOT_KEY]);
    for (const p of matchPaths) {
      keys.add(parentKey(p));
      keys.add(pathKey(p));
    }

    const next: Record<string, GitDirEntry[]> = {};
    for (const key of keys) {
      next[key] = childEntries(allDirs, key).filter(
        (c) => matchPaths.has(c.path) || allDirs.some((d) => d.path.startsWith(`${c.path}/`) && matchPaths.has(d.path)),
      );
    }
    return next;
  }, [filtering, allDirs, childrenByPath, filterQ]);

  const filterExpanded = useMemo(() => {
    if (!filtering || !allDirs) return expanded;
    const next = new Set<string>([ROOT_KEY]);
    for (const item of allDirs) {
      if (item.path.toLowerCase().includes(filterQ) || item.name.toLowerCase().includes(filterQ)) {
        for (const a of ancestorsOf(item.path)) {
          next.add(parentKey(a));
        }
      }
    }
    return next;
  }, [filtering, allDirs, filterQ, expanded]);

  const pathLabel = selected === "." ? "Repository root" : selected;
  const busy = loadingPaths.size > 0 || loadingAll;

  const quickPicks = useMemo(() => {
    const picks: { label: string; path: string }[] = [{ label: "Repository root", path: "." }];
    for (const w of workspaces) {
      const n = normalizeBlockRoot(w);
      if (n === ".") continue;
      picks.push({ label: n, path: n });
    }
    return picks;
  }, [workspaces]);

  function commit(root: string) {
    onChange(normalizeBlockRoot(root));
    onClose();
  }

  function renderNodes(parent: string, depth: number): ReactNode {
    const key = pathKey(parent);
    const items = visibleChildren[key];
    const isExpanded = filterExpanded.has(key);

    if (!isExpanded && key !== ROOT_KEY) return null;
    if (items === undefined) {
      if (loadingPaths.has(key) || (filtering && loadingAll)) {
        return (
          <p className="subtle repo-folder-tree__hint" style={{ paddingLeft: 12 + depth * 16 }}>
            Loading…
          </p>
        );
      }
      return null;
    }
    if (items.length === 0 && key === ROOT_KEY && !busy) {
      return <p className="subtle repo-folder-tree__hint">No folders in this repository.</p>;
    }

    return items.map((item) => {
      const childKey = item.path;
      const open = filterExpanded.has(childKey);
      const hasLoaded = visibleChildren[childKey] !== undefined;
      const childCount = visibleChildren[childKey]?.length;
      const canExpand = filtering
        ? (childCount ?? 0) > 0
        : !hasLoaded || (childCount !== undefined && childCount > 0);
      const isSelected = normalizeBlockRoot(selected) === normalizeBlockRoot(item.path);
      const rowBusy = loadingPaths.has(childKey);

      return (
        <div key={item.path} className="repo-folder-tree__branch">
          <div
            className={`repo-folder-tree__row${isSelected ? " selected" : ""}`}
            style={{ paddingLeft: 4 + depth * 16 }}
            role="treeitem"
            aria-expanded={canExpand ? open : undefined}
            aria-selected={isSelected}
          >
            <button
              type="button"
              className="repo-folder-tree__chev"
              aria-label={open ? `Collapse ${item.name}` : `Expand ${item.name}`}
              disabled={filtering || rowBusy || (!canExpand && hasLoaded)}
              onClick={(e) => {
                e.stopPropagation();
                void toggleExpand(item.path);
              }}
            >
              {rowBusy ? (
                <span className="repo-folder-tree__spinner" aria-hidden />
              ) : open ? (
                <IconChevronDown />
              ) : (
                <IconChevronRight />
              )}
            </button>
            <button
              type="button"
              className="repo-folder-tree__label"
              onClick={() => setSelected(item.path)}
              onDoubleClick={() => commit(item.path)}
            >
              <span className={`mac-folder${open ? " open" : ""}`} aria-hidden>
                {open ? <IconFolderOpen /> : <IconFolder />}
              </span>
              <span className="repo-folder-tree__name">{item.name}</span>
            </button>
          </div>
          {open && renderNodes(item.path, depth + 1)}
        </div>
      );
    });
  }

  return (
    <div className="repo-folder-picker" role="dialog" aria-label="Choose root folder">
      <div className="repo-folder-picker__head">
        <div className="repo-folder-picker__title">Choose root folder</div>
        <button type="button" className="button ghost sm" onClick={onClose}>
          Cancel
        </button>
      </div>

      {quickPicks.length > 1 && (
        <div className="repo-folder-picker__shortcuts" aria-label="Suggested folders">
          {quickPicks.map((p) => (
            <button
              key={p.path}
              type="button"
              className={`chip${normalizeBlockRoot(selected) === normalizeBlockRoot(p.path) ? " selected" : ""}`}
              onClick={() => setSelected(normalizeBlockRoot(p.path))}
            >
              {p.label}
            </button>
          ))}
        </div>
      )}

      <input
        type="search"
        className={`repo-folder-picker__filter${busy ? " awaiting" : ""}`}
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder={busy ? "Loading folders…" : "Filter folders…"}
        aria-label="Filter folders"
        aria-busy={busy || undefined}
        disabled={busy && !filtering}
      />

      <div
        className={`repo-folder-tree awaiting-box${busy ? " awaiting" : ""}`}
        role="tree"
        aria-label="Repository folders"
        aria-busy={busy || undefined}
      >
        <div
          className={`repo-folder-tree__row${selected === "." ? " selected" : ""}`}
          style={{ paddingLeft: 4 }}
          role="treeitem"
          aria-selected={selected === "."}
        >
          <span className="repo-folder-tree__chev repo-folder-tree__chev--spacer" aria-hidden />
          <button
            type="button"
            className="repo-folder-tree__label"
            onClick={() => setSelected(".")}
            onDoubleClick={() => commit(".")}
          >
            <span className="mac-folder open" aria-hidden>
              <IconFolderOpen />
            </span>
            <span className="repo-folder-tree__name">Repository root</span>
          </button>
        </div>
        {renderNodes(ROOT_KEY, 0)}
        {filtering && !loadingAll && allDirs && visibleChildren[ROOT_KEY]?.length === 0 && (
          <p className="subtle repo-folder-tree__hint">No folders match.</p>
        )}
      </div>

      <div className="repo-folder-picker__foot">
        <code className={`repo-folder-picker__selected${busy ? " awaiting" : ""}`} title={pathLabel}>{pathLabel}</code>
        <button
          type="button"
          className={`button sm${busy ? " awaiting" : ""}`}
          onClick={() => commit(selected)}
          disabled={busy}
        >
          Use this folder
        </button>
      </div>

      {showManual ? (
        <div className="repo-folder-picker__manual">
          <input
            value={manual}
            onChange={(e) => setManual(e.target.value)}
            placeholder="e.g. backend/cmd"
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
      ) : (
        <button
          type="button"
          className="button ghost sm repo-folder-picker__manual-toggle"
          onClick={() => setShowManual(true)}
        >
          Type a path instead
        </button>
      )}

      {error && <p className="form-error" style={{ marginTop: 8 }}>{error}</p>}
    </div>
  );
}
