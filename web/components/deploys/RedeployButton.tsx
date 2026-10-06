"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { IconPlay } from "@/components/icons";
import { SearchableSelect } from "@/components/SearchableSelect";
import { listRefs } from "@/lib/git-detect";
import type { SelectOption } from "@/lib/ui-helpers";

type Props = {
  projectId: string;
  /** Git provider (github/gitlab). When set with repo, loads branch/tag/release refs. */
  provider?: string;
  repo?: string;
  defaultBranch?: string;
  /** Compact single-button mode (env bar). Default shows the searchable ref picker. */
  compact?: boolean;
};

function kindLabel(kind: string): string {
  if (kind === "release") return "Release";
  if (kind === "tag") return "Tag";
  return "Branch";
}

export function RedeployButton({
  projectId,
  provider,
  repo,
  defaultBranch = "main",
  compact = false,
}: Props) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [refName, setRefName] = useState(defaultBranch);
  const [options, setOptions] = useState<SelectOption[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);

  const canLoadRefs = !!(provider && repo && (provider === "github" || provider === "gitlab"));

  useEffect(() => {
    setRefName(defaultBranch);
  }, [defaultBranch]);

  useEffect(() => {
    if (!canLoadRefs || compact) {
      setOptions([]);
      return;
    }
    let live = true;
    setLoadingRefs(true);
    listRefs(provider!, repo!)
      .then((items) => {
        if (!live) return;
        const opts: SelectOption[] = items.map((r) => ({
          value: r.name,
          label: r.default
            ? `${kindLabel(r.kind)} · ${r.name} (default)`
            : `${kindLabel(r.kind)} · ${r.name}`,
        }));
        setOptions(opts);
        if (!opts.some((o) => o.value === refName) && defaultBranch) {
          setRefName(defaultBranch);
        }
      })
      .catch(() => {
        if (live) setOptions([]);
      })
      .finally(() => {
        if (live) setLoadingRefs(false);
      });
    return () => {
      live = false;
    };
    // Intentionally omit refName so we don't re-fetch while typing a custom ref.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canLoadRefs, provider, repo, compact, defaultBranch]);

  const selectOptions = useMemo(() => {
    if (options.length > 0) return options;
    return [{ value: defaultBranch, label: `Branch · ${defaultBranch}` }];
  }, [options, defaultBranch]);

  async function run() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${projectId}/runs`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          trigger: "manual",
          branch: (refName || defaultBranch).trim() || defaultBranch,
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      const run = (await res.json()) as { id: string };
      router.push(`/deploys/runs/${run.id}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (compact) {
    return (
      <button type="button" className="button" disabled={busy} onClick={() => void run()}>
        <IconPlay /> {busy ? "Starting…" : "Redeploy"}
      </button>
    );
  }

  return (
    <div className="redeploy-controls">
      {canLoadRefs && (
        <div className="field" style={{ margin: 0, minWidth: 220, flex: 1 }}>
          <label htmlFor={`redeploy-ref-${projectId}`} style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)" }}>
            Branch, tag, or release
          </label>
          <SearchableSelect
            id={`redeploy-ref-${projectId}`}
            value={refName}
            options={selectOptions}
            allowCustom
            placeholder={loadingRefs ? "Loading refs…" : "Branch, tag, or release"}
            onChange={setRefName}
            aria-label="Redeploy from branch, tag, or release"
            disabled={busy || loadingRefs}
          />
        </div>
      )}
      <button type="button" className="button" disabled={busy} onClick={() => void run()}>
        <IconPlay /> {busy ? "Starting…" : "Redeploy"}
      </button>
      {error && <p className="form-error" style={{ margin: 0, flexBasis: "100%" }}>{error}</p>}
    </div>
  );
}
