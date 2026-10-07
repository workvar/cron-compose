"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { DeployApp, DeployEnvVar } from "@/lib/types";
import { parseDotEnv } from "@/lib/parse-dotenv";
import { IconPlus } from "@/components/icons";

export type EnvRow = {
  id: string;
  key: string;
  value: string;
  sensitive: boolean;
  hasValue: boolean;
  replacing: boolean;
};

function rowId() {
  return `e-${Math.random().toString(36).slice(2, 10)}`;
}

export function envVarsToRows(vars: DeployEnvVar[] | undefined): EnvRow[] {
  return (vars ?? []).map((v) => ({
    id: rowId(),
    key: v.key,
    value: v.sensitive ? "" : (v.value ?? ""),
    sensitive: !!v.sensitive,
    hasValue: !!v.has_value || (!!v.value && !v.sensitive),
    replacing: false,
  }));
}

export function rowsToEnvVars(rows: EnvRow[]): DeployEnvVar[] {
  return rows
    .filter((r) => r.key.trim())
    .map((r) => ({
      key: r.key.trim(),
      value: r.sensitive && !r.replacing && r.hasValue && r.value === "" ? "" : r.value,
      sensitive: r.sensitive,
      has_value: r.hasValue || r.value !== "",
    }));
}

function appsSignature(apps: DeployApp[]): string {
  return apps
    .map((a) => `${a.name}|${a.root}|${JSON.stringify(a.env ?? [])}`)
    .join("||");
}

function varsSignature(vars: DeployEnvVar[] | undefined): string {
  return JSON.stringify(vars ?? []);
}

type EnvTableProps = {
  pasteId: string;
  paste: string;
  onPaste: (text: string) => void;
  onApplyPaste: () => void;
  rows: EnvRow[];
  onPatch: (mut: (rows: EnvRow[]) => EnvRow[]) => void;
  /** Shared/global env is stored as a plain map — no sensitive secrets. */
  allowSensitive?: boolean;
};

function EnvTable({
  pasteId,
  paste,
  onPaste,
  onApplyPaste,
  rows,
  onPatch,
  allowSensitive = true,
}: EnvTableProps) {
  return (
    <>
      <div className="field" style={{ marginTop: 12 }}>
        <label htmlFor={pasteId}>Paste .env</label>
        <textarea
          id={pasteId}
          rows={3}
          placeholder={"# comments ignored\nKEY=value"}
          value={paste}
          onChange={(e) => onPaste(e.target.value)}
        />
        <div className="row" style={{ marginTop: 8, justifyContent: "flex-end" }}>
          <button
            type="button"
            className="button secondary sm"
            disabled={!paste.trim()}
            onClick={onApplyPaste}
          >
            Parse &amp; add
          </button>
        </div>
      </div>

      <div className="env-table" style={{ marginTop: 12 }}>
        {rows.length === 0 && (
          <p className="subtle" style={{ fontSize: 13 }}>No variables yet.</p>
        )}
        {rows.map((row) => (
          <div className={`env-row${allowSensitive ? "" : " env-row-plain"}`} key={row.id}>
            <input
              aria-label="Key"
              placeholder="KEY"
              value={row.key}
              onChange={(e) =>
                onPatch((rs) =>
                  rs.map((r) => (r.id === row.id ? { ...r, key: e.target.value } : r)),
                )
              }
            />
            {allowSensitive && row.sensitive && row.hasValue && !row.replacing ? (
              <div className="env-secret">
                <span className="subtle">••••••••</span>
                <button
                  type="button"
                  className="button ghost sm"
                  onClick={() =>
                    onPatch((rs) =>
                      rs.map((r) =>
                        r.id === row.id ? { ...r, replacing: true, value: "" } : r,
                      ),
                    )
                  }
                >
                  Replace
                </button>
              </div>
            ) : (
              <input
                aria-label="Value"
                placeholder={row.sensitive ? "New secret value" : "value"}
                type={row.sensitive ? "password" : "text"}
                value={row.value}
                onChange={(e) =>
                  onPatch((rs) =>
                    rs.map((r) =>
                      r.id === row.id
                        ? { ...r, value: e.target.value, hasValue: true }
                        : r,
                    ),
                  )
                }
              />
            )}
            {allowSensitive && (
              <label className="env-sensitive">
                <input
                  type="checkbox"
                  checked={row.sensitive}
                  onChange={(e) =>
                    onPatch((rs) =>
                      rs.map((r) =>
                        r.id === row.id
                          ? {
                              ...r,
                              sensitive: e.target.checked,
                              replacing: e.target.checked ? r.replacing || !!r.value : false,
                            }
                          : r,
                      ),
                    )
                  }
                />
                Sensitive
              </label>
            )}
            <button
              type="button"
              className="button ghost sm"
              aria-label={`Remove ${row.key || "variable"}`}
              onClick={() => onPatch((rs) => rs.filter((r) => r.id !== row.id))}
            >
              ×
            </button>
          </div>
        ))}
      </div>

      <button
        type="button"
        className="button secondary sm"
        style={{ marginTop: 10 }}
        onClick={() =>
          onPatch((rs) => [
            ...rs,
            { id: rowId(), key: "", value: "", sensitive: false, hasValue: false, replacing: false },
          ])
        }
      >
        <IconPlus /> Add variable
      </button>
    </>
  );
}

type Props = {
  apps: DeployApp[];
  onChange: (apps: DeployApp[]) => void;
  /** Shared across every process; process-level values win on the same key. */
  globalEnv?: DeployEnvVar[];
  onGlobalEnv?: (env: DeployEnvVar[]) => void;
  onAutosave?: (apps: DeployApp[], globalEnv?: DeployEnvVar[]) => Promise<void>;
  autosaveMs?: number;
  title?: string;
};

export function AppEnvEditor({
  apps,
  onChange,
  globalEnv,
  onGlobalEnv,
  onAutosave,
  autosaveMs = 700,
  title = "Environment variables",
}: Props) {
  const showGlobal = !!onGlobalEnv;
  const [globalRows, setGlobalRows] = useState<EnvRow[]>(() => envVarsToRows(globalEnv));
  const [rowsByApp, setRowsByApp] = useState<Record<string, EnvRow[]>>(() => {
    const m: Record<string, EnvRow[]> = {};
    for (const a of apps) m[a.name] = envVarsToRows(a.env);
    return m;
  });
  const [paste, setPaste] = useState<Record<string, string>>({});
  const [status, setStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const appsSigRef = useRef(appsSignature(apps));
  const globalSigRef = useRef(varsSignature(globalEnv));
  const globalRowsRef = useRef(globalRows);
  const rowsByAppRef = useRef(rowsByApp);
  globalRowsRef.current = globalRows;
  rowsByAppRef.current = rowsByApp;

  // Re-hydrate when parent loads a different project snapshot (not our own echo).
  useEffect(() => {
    const sig = appsSignature(apps);
    if (sig === appsSigRef.current) return;
    appsSigRef.current = sig;
    const m: Record<string, EnvRow[]> = {};
    for (const a of apps) m[a.name] = envVarsToRows(a.env);
    setRowsByApp(m);
  }, [apps]);

  useEffect(() => {
    if (!showGlobal) return;
    const sig = varsSignature(globalEnv);
    if (sig === globalSigRef.current) return;
    globalSigRef.current = sig;
    setGlobalRows(envVarsToRows(globalEnv));
  }, [globalEnv, showGlobal]);

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  const scheduleAutosave = useCallback(
    (nextApps: DeployApp[], nextGlobal?: DeployEnvVar[]) => {
      if (!onAutosave) return;
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => {
        setStatus("saving");
        setError(null);
        void onAutosave(nextApps, nextGlobal)
          .then(() => {
            setStatus("saved");
            appsSigRef.current = appsSignature(nextApps);
            if (nextGlobal) globalSigRef.current = varsSignature(nextGlobal);
          })
          .catch((e) => {
            setStatus("error");
            setError((e as Error).message);
          });
      }, autosaveMs);
    },
    [onAutosave, autosaveMs],
  );

  const emitApps = useCallback(
    (nextRows: Record<string, EnvRow[]>) => {
      const nextApps = apps.map((a) => ({
        ...a,
        env: rowsToEnvVars(nextRows[a.name] ?? []),
      }));
      appsSigRef.current = appsSignature(nextApps);
      onChange(nextApps);
      scheduleAutosave(
        nextApps,
        showGlobal ? rowsToEnvVars(globalRowsRef.current) : undefined,
      );
    },
    [apps, onChange, scheduleAutosave, showGlobal],
  );

  const emitGlobal = useCallback(
    (nextRows: EnvRow[]) => {
      if (!onGlobalEnv) return;
      const next = rowsToEnvVars(nextRows).map((v) => ({ ...v, sensitive: false }));
      globalSigRef.current = varsSignature(next);
      onGlobalEnv(next);
      const nextApps = apps.map((a) => ({
        ...a,
        env: rowsToEnvVars(rowsByAppRef.current[a.name] ?? []),
      }));
      scheduleAutosave(nextApps, next);
    },
    [onGlobalEnv, apps, scheduleAutosave],
  );

  function patchRows(appName: string, mut: (rows: EnvRow[]) => EnvRow[]) {
    setRowsByApp((prev) => {
      const next = { ...prev, [appName]: mut(prev[appName] ?? []) };
      emitApps(next);
      return next;
    });
  }

  function patchGlobal(mut: (rows: EnvRow[]) => EnvRow[]) {
    setGlobalRows((prev) => {
      const next = mut(prev);
      emitGlobal(next);
      return next;
    });
  }

  function applyPaste(scope: string) {
    const text = paste[scope] ?? "";
    const parsed = parseDotEnv(text);
    if (parsed.length === 0) return;
    const merge = (rows: EnvRow[]) => {
      const byKey = new Map(rows.map((r) => [r.key, r]));
      for (const p of parsed) {
        const prev = byKey.get(p.key);
        byKey.set(p.key, {
          id: prev?.id ?? rowId(),
          key: p.key,
          value: p.value,
          sensitive: scope === "global" ? false : (prev?.sensitive ?? false),
          hasValue: true,
          replacing: scope !== "global" && prev?.sensitive ? true : false,
        });
      }
      return Array.from(byKey.values());
    };
    if (scope === "global") patchGlobal(merge);
    else patchRows(scope, merge);
    setPaste((p) => ({ ...p, [scope]: "" }));
  }

  if (apps.length === 0 && !showGlobal) {
    return (
      <div className="panel">
        <div className="card-title">{title}</div>
        <p className="subtle" style={{ marginTop: 8 }}>Select an app to configure environment variables.</p>
      </div>
    );
  }

  const statusPill = onAutosave ? (
    <span className="pill">
      {status === "saving" ? "Saving…" : status === "saved" ? "Saved" : status === "error" ? "Error" : "Autosave"}
    </span>
  ) : null;

  return (
    <div className="stack">
      {showGlobal && (
        <div className="panel">
          <div className="row" style={{ alignItems: "flex-start" }}>
            <div>
              {title && <div className="card-title">{title}</div>}
              <p className="subtle" style={{ margin: "4px 0 0", fontSize: 13 }}>
                <strong>Shared (all processes)</strong>
              </p>
              <p className="field-hint" style={{ marginTop: 6, marginBottom: 0 }}>
                Applied to every process. A process can override the same key with its own value.
                Secrets stay on individual processes.
              </p>
            </div>
            {statusPill}
          </div>
          <EnvTable
            pasteId="paste-global"
            paste={paste.global ?? ""}
            onPaste={(text) => setPaste((p) => ({ ...p, global: text }))}
            onApplyPaste={() => applyPaste("global")}
            rows={globalRows}
            onPatch={patchGlobal}
            allowSensitive={false}
          />
        </div>
      )}

      {apps.map((app) => {
        const rows = rowsByApp[app.name] ?? [];
        return (
          <div className="panel" key={`${app.name}-${app.root}`}>
            <div className="row" style={{ alignItems: "flex-start" }}>
              <div>
                {!showGlobal && title && <div className="card-title">{title}</div>}
                <p className="subtle" style={{ margin: showGlobal || !title ? "0" : "4px 0 0", fontSize: 13 }}>
                  <strong>{app.name}</strong>
                  <span className="subtle"> · </span>
                  <code>{app.root}</code>
                  {showGlobal && <span className="subtle"> — this process only</span>}
                </p>
              </div>
              {!showGlobal && statusPill}
            </div>

            <EnvTable
              pasteId={`paste-${app.name}`}
              paste={paste[app.name] ?? ""}
              onPaste={(text) => setPaste((p) => ({ ...p, [app.name]: text }))}
              onApplyPaste={() => applyPaste(app.name)}
              rows={rows}
              onPatch={(mut) => patchRows(app.name, mut)}
            />
          </div>
        );
      })}
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}
