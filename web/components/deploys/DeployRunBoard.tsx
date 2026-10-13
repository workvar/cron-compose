"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  buildDeployLive,
  type LiveStep,
  type LiveView,
  type ProgressApp,
  type StepState,
} from "@/lib/deploy-progress";
import type { DeployInventory, DeployProject, LogLine } from "@/lib/types";
import { buildProcessRows, type ProcessRow } from "@/lib/project-processes";
import { TerminalFrame } from "@/components/terminal/TerminalFrame";
import { ProcessTaskActions } from "@/components/deploys/ProcessTaskActions";

type Props = {
  logs: LogLine[];
  project: DeployProject | null;
  status: string;
  live: boolean;
};

function appsOf(project: DeployProject | null): ProgressApp[] {
  if (!project) return [];
  const apps = project.apps || [];
  if (apps.length > 0) {
    return apps.map((a) => ({
      name: a.name || project.name || "app",
      root: a.root || project.root_directory,
      language: a.language || project.language,
      install: a.install || project.install_script,
      cleanup: a.cleanup,
      preStart: a.pre_start,
      processManager: a.process_manager || project.process_manager,
      healthPath: a.health?.path || (apps.length === 1 ? project.health_path : ""),
    }));
  }
  return [{
    name: project.name || "app",
    root: project.root_directory,
    language: project.language,
    install: project.install_script,
    processManager: project.process_manager,
    healthPath: project.health_path,
  }];
}

function singleRail(view: LiveView): LiveStep[] {
  const proc = view.processes[0];
  if (!proc) return view.shared;
  const early = proc.steps.filter((s) => s.kind !== "start" && s.kind !== "health");
  const late = proc.steps.filter((s) => s.kind === "start" || s.kind === "health");
  const release = view.shared.find((s) => s.kind === "release");
  const prepare = view.shared.filter((s) => s.kind !== "release");
  return [...prepare, ...early, ...(release ? [release] : []), ...late];
}

function paneText(view: LiveView, pane: string, everything: boolean): string {
  const rows = everything ? view.lines : view.lines.filter((l) => l.pane === pane);
  return rows.map((l) => l.text).join("\n");
}

function defaultOpen(id: string, view: LiveView): boolean {
  if (id === "shared") {
    // The server log is the whole story until an app starts producing output.
    const appStarted = view.processes.some((p) => p.state !== "pending");
    return !appStarted;
  }
  return true;
}

function rowForProcess(rows: ProcessRow[], name: string): ProcessRow | null {
  return (
    rows.find(
      (r) =>
        r.label === name ||
        r.processName === name ||
        r.processName.endsWith(`-${name.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`),
    ) ?? null
  );
}

export function DeployRunBoard({ logs, project, status, live }: Props) {
  const apps = useMemo(() => appsOf(project), [project]);
  const view = useMemo(
    () => buildDeployLive({
      logs,
      apps,
      projectName: project?.name,
      status,
    }),
    [logs, apps, project?.name, status],
  );
  const multi = view.processes.length > 1;
  const [forced, setForced] = useState<Record<string, boolean>>({});
  const [inventory, setInventory] = useState<DeployInventory | null>(null);

  const refreshInventory = useCallback(async () => {
    if (!project?.server_id) return;
    try {
      const res = await fetch(
        `/api/servers/${encodeURIComponent(project.server_id)}/deploy-inventory`,
      );
      if (!res.ok) return;
      setInventory((await res.json()) as DeployInventory);
    } catch {
      /* ignore — controls stay disabled until inventory is available */
    }
  }, [project?.server_id]);

  useEffect(() => {
    void refreshInventory();
  }, [refreshInventory, status]);

  // Re-poll while the run is live so controls light up once pm2/systemd/docker
  // registers the process after the start step.
  useEffect(() => {
    if (!live || !project?.server_id) return;
    const id = window.setInterval(() => void refreshInventory(), 5000);
    return () => window.clearInterval(id);
  }, [live, project?.server_id, refreshInventory]);

  const processRows = useMemo(
    () => (project ? buildProcessRows(project, inventory) : []),
    [project, inventory],
  );
  const manageable = processRows.some((r) => r.pm !== "none");

  function isOpen(id: string) {
    if (id in forced) return forced[id];
    return defaultOpen(id, view);
  }

  function toggle(id: string) {
    setForced((prev) => ({ ...prev, [id]: !isOpen(id) }));
  }

  const rail = multi ? view.shared : singleRail(view);

  return (
    <div className="deploy-live">
      <section className="panel deploy-live-summary">
        <div className="card-head">
          <div>
            <div className="card-title">Progress</div>
            <div className="deploy-live-headline">{view.headline}</div>
          </div>
          <div className="deploy-live-pct" aria-hidden="true">{view.percent}%</div>
        </div>
        <div
          className="deploy-meter"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={view.percent}
          aria-valuetext={`${view.percent}% ${view.headline}`}
        >
          <span style={{ width: `${view.percent}%` }} />
        </div>
        <Rail steps={rail} />
      </section>

      <div className={`deploy-proc-grid${multi ? " is-multi" : ""}`}>
        {multi && (
          <div className="deploy-shared-term">
            <LogTerminal
              title="Server"
              text={paneText(view, "shared", false)}
              open={isOpen("shared")}
              onToggle={() => toggle("shared")}
              live={live}
              tall={false}
            />
          </div>
        )}
        {view.processes.map((proc) => {
          const row = manageable ? rowForProcess(processRows, proc.name) : null;
          const showControls = Boolean(row && row.pm !== "none");
          return (
            <section key={proc.id} className={multi ? "panel deploy-proc" : "deploy-proc"}>
              {multi ? (
                <>
                  <div className="deploy-proc-top">
                    <div>
                      <div className="deploy-proc-name">{proc.name}</div>
                      <div className={`deploy-proc-state is-${proc.state}`}>{stateLabel(proc.state)}</div>
                    </div>
                    <div className="deploy-live-pct sm" aria-hidden="true">{proc.percent}%</div>
                  </div>
                  {showControls && (
                    <div className="deploy-proc-actions-wrap">
                      <ProcessTaskActions
                        row={row}
                        enabled={!live || proc.state === "done" || proc.state === "failed"}
                        onChanged={() => void refreshInventory()}
                      />
                    </div>
                  )}
                  <div
                    className="deploy-meter"
                    role="progressbar"
                    aria-label={proc.name}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={proc.percent}
                  >
                    <span style={{ width: `${proc.percent}%` }} />
                  </div>
                  <Rail steps={proc.steps} />
                </>
              ) : (
                showControls && (
                  <div className="deploy-proc-actions-wrap">
                    <ProcessTaskActions
                      row={row}
                      enabled={!live || proc.state === "done" || proc.state === "failed"}
                      onChanged={() => void refreshInventory()}
                    />
                  </div>
                )
              )}
              <LogTerminal
                title={proc.name}
                text={paneText(view, proc.id, !multi)}
                open={isOpen(proc.id)}
                onToggle={() => toggle(proc.id)}
                live={live && (proc.state === "active" || proc.state === "pending" || !multi)}
                tall={!multi}
              />
            </section>
          );
        })}
      </div>
    </div>
  );
}

function stateLabel(state: StepState): string {
  switch (state) {
    case "active":
      return "In progress";
    case "done":
      return "Done";
    case "failed":
      return "Failed";
    case "skipped":
      return "Skipped";
    default:
      return "Waiting";
  }
}

function Rail({ steps }: { steps: LiveStep[] }) {
  if (steps.length === 0) return null;
  return (
    <ol className="deploy-rail">
      {steps.map((s, i) => (
        <li key={s.id} className={`deploy-rail-step is-${s.state}`} title={s.detail || s.label}>
          {i > 0 && <span className="deploy-rail-sep" aria-hidden="true">›</span>}
          <StepMark state={s.state} />
          <span className="deploy-rail-label">{s.label}</span>
          {s.state === "active" && <span className="deploy-rail-pct">{s.percent}%</span>}
        </li>
      ))}
    </ol>
  );
}

function StepMark({ state }: { state: StepState }) {
  return (
    <span className={`deploy-mark is-${state}`} aria-hidden="true">
      {state === "done" ? "✓" : state === "failed" ? "!" : state === "skipped" ? "–" : ""}
    </span>
  );
}

function LogTerminal({
  title,
  text,
  open,
  onToggle,
  live,
  tall,
}: {
  title: string;
  text: string;
  open: boolean;
  onToggle: () => void;
  live: boolean;
  tall: boolean;
}) {
  const ref = useRef<HTMLPreElement>(null);
  const stick = useRef(true);
  const count = text ? text.split("\n").filter((l) => l.trim()).length : 0;

  useEffect(() => {
    const el = ref.current;
    if (!open || !el || !stick.current) return;
    el.scrollTop = el.scrollHeight;
  }, [text, open]);

  return (
    <TerminalFrame
      title={`${title} · ${count} ${count === 1 ? "line" : "lines"}`}
      actions={
        <button type="button" className="deploy-term-collapse" aria-expanded={open} onClick={onToggle}>
          {open ? "Hide log" : "Show log"}
        </button>
      }
    >
      {open ? (
        <pre
          ref={ref}
          className={`term-log${tall ? "" : " is-short"}`}
          onScroll={(e) => {
            const el = e.currentTarget;
            stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
          }}
        >
          {text.trim() ? (
            text
          ) : (
            <span className="term-log-empty">{live ? "(waiting for output…)" : "(no output)"}</span>
          )}
        </pre>
      ) : null}
    </TerminalFrame>
  );
}
