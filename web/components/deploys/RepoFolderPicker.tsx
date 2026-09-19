"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { GitDirEntry, GitDirList } from "@/lib/types";
import { normalizeBlockRoot } from "@/lib/project-blocks";

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

function breadcrumbSegments(path: string): { label: string; path: string }[] {
  const segs = [{ label: "repo root", path: "" }];
  if (!path) return segs;
  const parts = path.split("/").filter(Boolean);
  let acc = "";
  for (const part of parts) {
    acc = acc ? `${acc}/${part}` : part;
    segs.push({ label: part, path: acc });
  }
  return segs;
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
  const initialBrowse = value && value !== "." ? value : "";
  const [browsePath, setBrowsePath] = useState(initialBrowse);
  const [items, setItems] = useState<GitDirEntry[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<"lazy" | "tree">("lazy");
  const [treeQuery, setTreeQuery] = useState("");
  const [manual, setManual] = useState("");

  const fetchDirs = useCallback(
    async (path: string, recursive: boolean) => {
      setLoading(true);
      setError(null);
      try {
        const q = new URLSearchParams({
          provider,
          repo,
          branch,
          path,
        });
        if (recursive) q.set("recursive", "1");
        const res = await fetch(`/api/git/dirs?${q}`);
        if (!res.ok) throw new Error(await res.text());
        const data = (await res.json()) as GitDirList;
        setItems(data.items ?? []);
        setTruncated(!!data.truncated);
      } catch (e) {
        setError((e as Error).message);
        setItems([]);
        setTruncated(false);
      } finally {
        setLoading(false);
      }
    },
    [provider, repo, branch],
  );

  useEffect(() => {
    if (mode === "lazy") {
      void fetchDirs(browsePath, false);
    }
  }, [mode, browsePath, fetchDirs]);

  useEffect(() => {
    if (mode === "tree") {
      void fetchDirs("", true);
    }
  }, [mode, fetchDirs]);

  const crumbs = useMemo(() => breadcrumbSegments(browsePath), [browsePath]);

  const filteredTree = useMemo(() => {
    const q = treeQuery.trim().toLowerCase();
    if (!q) return items;
    return items.filter(
      (item) =>
        item.path.toLowerCase().includes(q) || item.name.toLowerCase().includes(q),
    );
  }, [items, treeQuery]);

  function commitRoot(root: string) {
    onChange(normalizeBlockRoot(root));
    onClose();
  }

  function useCurrentFolder() {
    commitRoot(browsePath === "" ? "." : browsePath);
  }

  return (
    <div className="panel" style={{ marginTop: 8 }}>
      <div className="row" style={{ justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ fontWeight: 700 }}>Choose folder</div>
        <button type="button" className="button ghost sm" onClick={onClose}>
          Close
        </button>
      </div>

      {workspaces.length > 0 && (
        <div className="field" style={{ marginTop: 12 }}>
          <label>Detected workspaces</label>
          <div className="chips">
            {workspaces.map((w) => (
              <button
                key={w}
                type="button"
                className={`chip${value === w ? " selected" : ""}`}
                onClick={() => commitRoot(w)}
              >
                <code>{w}</code>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="row" style={{ marginTop: 12, gap: 8 }}>
        <button
          type="button"
          className={`button secondary sm${mode === "lazy" ? " on" : ""}`}
          onClick={() => setMode("lazy")}
        >
          Browse
        </button>
        <button
          type="button"
          className={`button secondary sm${mode === "tree" ? " on" : ""}`}
          onClick={() => setMode("tree")}
        >
          Load full tree
        </button>
      </div>

      {mode === "lazy" && (
        <>
          <nav className="row" style={{ marginTop: 12, flexWrap: "wrap", gap: 4, fontSize: 13 }}>
            {crumbs.map((c, i) => (
              <span key={c.path || "root"}>
                {i > 0 && <span className="subtle"> / </span>}
                <button
                  type="button"
                  className="button ghost sm"
                  style={{ padding: "2px 6px", fontSize: 13 }}
                  onClick={() => setBrowsePath(c.path)}
                >
                  {c.label}
                </button>
              </span>
            ))}
          </nav>

          <div className="row" style={{ marginTop: 8, gap: 8 }}>
            <button
              type="button"
              className="button secondary sm"
              disabled={browsePath === ""}
              onClick={() => setBrowsePath(parentPath(browsePath))}
            >
              Up
            </button>
            <button type="button" className="button sm" onClick={useCurrentFolder}>
              Use this folder
            </button>
          </div>

          <div className="stack" style={{ marginTop: 12, maxHeight: 240, overflow: "auto" }}>
            {loading && <p className="subtle">Loading directories…</p>}
            {!loading && items.length === 0 && !error && (
              <p className="subtle">No subdirectories here.</p>
            )}
            {items.map((item) => (
              <button
                key={item.path}
                type="button"
                className="button secondary"
                style={{ textAlign: "left", justifyContent: "flex-start" }}
                onClick={() => setBrowsePath(item.path)}
              >
                <code>{item.name}/</code>
              </button>
            ))}
          </div>
        </>
      )}

      {mode === "tree" && (
        <>
          <div className="field" style={{ marginTop: 12 }}>
            <label htmlFor="tree-filter">Filter paths</label>
            <input
              id="tree-filter"
              value={treeQuery}
              onChange={(e) => setTreeQuery(e.target.value)}
              placeholder="Type to filter…"
            />
          </div>
          {truncated && (
            <p className="form-error" style={{ marginTop: 8 }}>
              Directory list was truncated (2000 entry cap). Use browse mode or type a path manually.
            </p>
          )}
          <div className="stack" style={{ marginTop: 12, maxHeight: 280, overflow: "auto" }}>
            {loading && <p className="subtle">Loading full tree…</p>}
            {!loading && (
              <button
                type="button"
                className="button secondary"
                style={{ textAlign: "left" }}
                onClick={() => commitRoot(".")}
              >
                <code>.</code> (repo root)
              </button>
            )}
            {filteredTree.map((item) => (
              <button
                key={item.path}
                type="button"
                className="button secondary"
                style={{ textAlign: "left", justifyContent: "flex-start" }}
                onClick={() => commitRoot(item.path)}
              >
                <code>{item.path}</code>
              </button>
            ))}
            {!loading && filteredTree.length === 0 && treeQuery && (
              <p className="subtle">No paths match.</p>
            )}
          </div>
        </>
      )}

      <div className="field" style={{ marginTop: 16 }}>
        <label htmlFor="manual-path">Or type a path</label>
        <div className="row" style={{ gap: 8 }}>
          <input
            id="manual-path"
            value={manual}
            onChange={(e) => setManual(e.target.value)}
            placeholder="apps/web"
          />
          <button
            type="button"
            className="button secondary"
            disabled={!manual.trim()}
            onClick={() => commitRoot(manual.trim())}
          >
            Apply
          </button>
        </div>
      </div>

      {error && <p className="form-error" style={{ marginTop: 8 }}>{error}</p>}
      {truncated && mode === "lazy" && (
        <p className="subtle" style={{ marginTop: 8 }}>
          Some directories were omitted (list capped at 2000).
        </p>
      )}
    </div>
  );
}
