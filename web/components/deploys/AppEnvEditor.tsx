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

type Props = {
  apps: DeployApp[];
  onChange: (apps: DeployApp[]) => void;
  onAutosave?: (apps: DeployApp[]) => Promise<void>;
  autosaveMs?: number;
  title?: string;
};

export function AppEnvEditor({
  apps,
  onChange,
  onAutosave,
  autosaveMs = 700,
  title = "Environment variables",
}: Props) {
  const [rowsByApp, setRowsByApp] = useState<Record<string, EnvRow[]>>(() => {
    const m: Record<string, EnvRow[]> = {};
    for (const a of apps) m[a.name] = envVarsToRows(a.env);
    return m;
  });
  const [paste, setPaste] = useState<Record<string, string>>({});
  const [status, setStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const sigRef = useRef(appsSignature(apps));

  // Re-hydrate when parent loads a different project snapshot (not our own echo).
  useEffect(() => {
    const sig = appsSignature(apps);
    if (sig === sigRef.current) return;
    sigRef.current = sig;
    const m: Record<string, EnvRow[]> = {};
    for (const a of apps) m[a.name] = envVarsToRows(a.env);
    setRowsByApp(m);
  }, [apps]);

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  const emit = useCallback(
    (nextRows: Record<string, EnvRow[]>) => {
      const nextApps = apps.map((a) => ({
        ...a,
        env: rowsToEnvVars(nextRows[a.name] ?? []),
      }));
      sigRef.current = appsSignature(nextApps);
      onChange(nextApps);
      if (!onAutosave) return;
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => {
        setStatus("saving");
        setError(null);
        void onAutosave(nextApps)
          .then(() => {
            setStatus("saved");
            sigRef.current = appsSignature(nextApps);
          })
          .catch((e) => {
            setStatus("error");
            setError((e as Error).message);
          });
      }, autosaveMs);
    },
    [apps, onAutosave, onChange, autosaveMs],
  );

  function patchRows(appName: string, mut: (rows: EnvRow[]) => EnvRow[]) {
    setRowsByApp((prev) => {
      const next = { ...prev, [appName]: mut(prev[appName] ?? []) };
      emit(next);
      return next;
    });
  }

  function applyPaste(appName: string) {
    const text = paste[appName] ?? "";
    const parsed = parseDotEnv(text);
    if (parsed.length === 0) return;
    patchRows(appName, (rows) => {
      const byKey = new Map(rows.map((r) => [r.key, r]));
      for (const p of parsed) {
        const prev = byKey.get(p.key);
        byKey.set(p.key, {
          id: prev?.id ?? rowId(),
          key: p.key,
          value: p.value,
          sensitive: prev?.sensitive ?? false,
          hasValue: true,
          replacing: prev?.sensitive ? true : false,
        });
      }
      return Array.from(byKey.values());
    });
    setPaste((p) => ({ ...p, [appName]: "" }));
  }

  if (apps.length === 0) {
    return (
      <div className="panel">
        <div className="card-title">{title}</div>
        <p className="subtle" style={{ marginTop: 8 }}>Select an app to configure environment variables.</p>
      </div>
    );
  }

  return (
    <div className="stack">
      {apps.map((app) => {
        const rows = rowsByApp[app.name] ?? [];
        return (
          <div className="panel" key={`${app.name}-${app.root}`}>
            <div className="row" style={{ alignItems: "flex-start" }}>
              <div>
                <div className="card-title">{title}</div>
                <p className="subtle" style={{ margin: "4px 0 0", fontSize: 13 }}>
                  <strong>{app.name}</strong>
                  <span className="subtle"> · </span>
                  <code>{app.root}</code>
                </p>
              </div>
              {onAutosave && (
                <span className="pill">
                  {status === "saving" ? "Saving…" : status === "saved" ? "Saved" : status === "error" ? "Error" : "Autosave"}
                </span>
              )}
            </div>

            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor={`paste-${app.name}`}>Paste .env</label>
              <textarea
                id={`paste-${app.name}`}
                rows={3}
                placeholder={"# comments ignored\nKEY=value"}
                value={paste[app.name] ?? ""}
                onChange={(e) => setPaste((p) => ({ ...p, [app.name]: e.target.value }))}
              />
              <div className="row" style={{ marginTop: 8, justifyContent: "flex-end" }}>
                <button
                  type="button"
                  className="button secondary sm"
                  disabled={!(paste[app.name] ?? "").trim()}
                  onClick={() => applyPaste(app.name)}
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
                <div className="env-row" key={row.id}>
                  <input
                    aria-label="Key"
                    placeholder="KEY"
                    value={row.key}
                    onChange={(e) =>
                      patchRows(app.name, (rs) =>
                        rs.map((r) => (r.id === row.id ? { ...r, key: e.target.value } : r)),
                      )
                    }
                  />
                  {row.sensitive && row.hasValue && !row.replacing ? (
                    <div className="env-secret">
                      <span className="subtle">••••••••</span>
                      <button
                        type="button"
                        className="button ghost sm"
                        onClick={() =>
                          patchRows(app.name, (rs) =>
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
                        patchRows(app.name, (rs) =>
                          rs.map((r) =>
                            r.id === row.id
                              ? { ...r, value: e.target.value, hasValue: true }
                              : r,
                          ),
                        )
                      }
                    />
                  )}
                  <label className="env-sensitive">
                    <input
                      type="checkbox"
                      checked={row.sensitive}
                      onChange={(e) =>
                        patchRows(app.name, (rs) =>
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
                  <button
                    type="button"
                    className="button ghost sm"
                    aria-label={`Remove ${row.key || "variable"}`}
                    onClick={() =>
                      patchRows(app.name, (rs) => rs.filter((r) => r.id !== row.id))
                    }
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
                patchRows(app.name, (rs) => [
                  ...rs,
                  { id: rowId(), key: "", value: "", sensitive: false, hasValue: false, replacing: false },
                ])
              }
            >
              <IconPlus /> Add variable
            </button>
          </div>
        );
      })}
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}
