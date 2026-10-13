"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  buildDeployLive,
  type LiveProcess,
  type LiveStep,
  type LiveView,
  type ProgressApp,
  type StepState,
} from "@/lib/deploy-progress";
import { activeWaveIndex, buildDeployWaves } from "@/lib/deploy-waves";
import type { Connector, DeployInventory, DeployProject, LogLine } from "@/lib/types";
import {
  buildProcessRows,
  isProcessRunning,
  type ProcessRow,
} from "@/lib/project-processes";
import { TerminalFrame } from "@/components/terminal/TerminalFrame";
import { ProcessTaskActions } from "@/components/deploys/ProcessTaskActions";
import { WaveVisualizer } from "@/components/deploys/WaveVisualizer";

type Props = {
  logs: LogLine[];
  project: DeployProject | null;
  status: string;
  live: boolean;
};

const PHASE_ORDER = [
  "install",
  "build",
  "cleanup",
  "pre_start",
  "run",
  "start",
  "health",
  "preflight",
  "clone",
  "release",
] as const;

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
      run: a.run,
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

function paneText(view: LiveView, pane: string, phase?: string): string {
  return view.lines
    .filter((l) => l.pane === pane && (!phase || phase === "all" || l.phase === phase))
    .map((l) => l.text)
    .join("\n");
}

function phasesForProcess(proc: LiveProcess): { id: string; label: string }[] {
  const seen = new Set<string>();
  const out: { id: string; label: string }[] = [{ id: "all", label: "All" }];
  for (const kind of PHASE_ORDER) {
    if (proc.steps.some((s) => s.kind === kind) && !seen.has(kind)) {
      seen.add(kind);
      out.push({ id: kind, label: kind.replace(/_/g, " ") });
    }
  }
  return out;
}

function activePhase(proc: LiveProcess): string {
  const active = proc.steps.find((s) => s.state === "active");
  if (active) return active.kind;
  const failed = proc.steps.find((s) => s.state === "failed");
  if (failed) return failed.kind;
  const lastDone = [...proc.steps].reverse().find((s) => s.state === "done");
  return lastDone?.kind || "all";
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
  const waves = useMemo(
    () => buildDeployWaves({
      flow: project?.flow,
      view,
      logs,
      status,
    }),
    [project?.flow, view, logs, status],
  );
  const waveIdx = useMemo(() => activeWaveIndex(waves), [waves]);

  const [focusPane, setFocusPane] = useState<string>("shared");
  const [phaseByPane, setPhaseByPane] = useState<Record<string, string>>({});
  const [inventory, setInventory] = useState<DeployInventory | null>(null);
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [controlsError, setControlsError] = useState<string | null>(null);

  useEffect(() => {
    const w = waves[waveIdx];
    if (!w) return;
    const active = w.tasks.find((t) => t.state === "active") || w.tasks[0];
    if (!active) return;
    setFocusPane(active.pane);
    setPhaseByPane((prev) => ({ ...prev, [active.pane]: active.phase }));
  }, [waveIdx, waves]);

  const refreshInventory = useCallback(async () => {
    if (!project?.server_id) return;
    const sid = encodeURIComponent(project.server_id);
    try {
      const [invRes, connRes] = await Promise.all([
        fetch(`/api/servers/${sid}/deploy-inventory`),
        fetch(`/api/servers/${sid}/connectors`),
      ]);
      if (invRes.ok) setInventory((await invRes.json()) as DeployInventory);
      if (connRes.ok) {
        const body = (await connRes.json()) as { items?: Connector[] };
        setConnectors(Array.isArray(body.items) ? body.items : []);
      }
      if (!invRes.ok && !connRes.ok) {
        setControlsError(`Could not load process controls (${invRes.status})`);
      } else {
        setControlsError(null);
      }
    } catch (e) {
      setControlsError((e as Error).message || "Could not load process controls");
    }
  }, [project?.server_id]);

  useEffect(() => {
    void refreshInventory();
  }, [refreshInventory, status]);

  useEffect(() => {
    if (!project?.server_id) return;
    if (!live && status !== "failed" && status !== "canceled" && status !== "agent_offline") {
      return;
    }
    const id = window.setInterval(() => void refreshInventory(), live ? 5000 : 15000);
    return () => window.clearInterval(id);
  }, [live, project?.server_id, refreshInventory, status]);

  const processRows = useMemo(
    () => (project ? buildProcessRows(project, inventory, connectors) : []),
    [project, inventory, connectors],
  );
  const manageable = processRows.some((r) => r.pm !== "none");
  const runFailed = status === "failed" || status === "canceled" || status === "agent_offline";

  const selectTask = useCallback((pane: string, phase: string) => {
    setFocusPane(pane);
    setPhaseByPane((prev) => ({ ...prev, [pane]: phase }));
  }, []);

  return (
    <div className="deploy-live">
      <section className={`panel deploy-live-summary${runFailed ? " is-failed" : ""}`}>
        <div className="card-head">
          <div>
            <div className="card-title">Progress</div>
            <div className="deploy-live-headline">{view.headline}</div>
          </div>
          <div className="deploy-live-pct" aria-hidden="true">{view.percent}%</div>
        </div>
        <div
          className={`deploy-meter${runFailed ? " is-failed" : ""}`}
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={view.percent}
          aria-valuetext={`${view.percent}% ${view.headline}`}
        >
          <span style={{ width: `${view.percent}%` }} />
        </div>
        <Rail steps={view.shared} />
      </section>

      <WaveVisualizer
        waves={waves}
        activeIndex={waveIdx}
        selectedPane={focusPane}
        selectedPhase={phaseByPane[focusPane]}
        onSelectTask={selectTask}
      />

      {controlsError && (
        <p className="form-error" style={{ margin: 0 }}>{controlsError}</p>
      )}

      <div
        className={`deploy-term-grid${view.processes.length > 1 ? " is-multi" : ""}`}
        style={{ ["--deploy-term-cols" as string]: Math.min(view.processes.length, 3) }}
      >
        <ProcessTerminal
          title="Server"
          pane="shared"
          view={view}
          steps={view.shared}
          phase={phaseByPane.shared || "all"}
          onPhase={(p) => setPhaseByPane((prev) => ({ ...prev, shared: p }))}
          focused={focusPane === "shared"}
          onFocus={() => setFocusPane("shared")}
          live={live}
        />
        {view.processes.map((proc) => {
          const row = manageable ? rowForProcess(processRows, proc.name) : null;
          const showControls = Boolean(row && row.pm !== "none");
          const startDone = proc.steps.some(
            (s) =>
              (s.kind === "start" || s.kind === "run") &&
              (s.state === "done" || s.state === "active"),
          );
          const knownRunning = Boolean(row?.resource && isProcessRunning(row.resource.state));
          const controlsEnabled =
            !live ||
            proc.state === "done" ||
            proc.state === "failed" ||
            knownRunning ||
            startDone ||
            Boolean(row?.resource);
          const phase = phaseByPane[proc.id] || activePhase(proc);
          return (
            <ProcessTerminal
              key={proc.id}
              title={proc.name}
              pane={proc.id}
              view={view}
              steps={proc.steps}
              phase={phase}
              onPhase={(p) => setPhaseByPane((prev) => ({ ...prev, [proc.id]: p }))}
              focused={focusPane === proc.id}
              onFocus={() => setFocusPane(proc.id)}
              live={live}
              state={proc.state}
              percent={proc.percent}
              pmState={row?.resource?.state}
              actions={
                showControls ? (
                  <ProcessTaskActions
                    row={row}
                    projectId={project?.id}
                    appName={proc.name}
                    serverId={project?.server_id}
                    runAsUser={project?.run_as_user}
                    enabled={controlsEnabled}
                    onChanged={() => void refreshInventory()}
                  />
                ) : null
              }
            />
          );
        })}
      </div>
    </div>
  );
}

function ProcessTerminal({
  title,
  pane,
  view,
  steps,
  phase,
  onPhase,
  focused,
  onFocus,
  live,
  state,
  percent,
  pmState,
  actions,
}: {
  title: string;
  pane: string;
  view: LiveView;
  steps: LiveStep[];
  phase: string;
  onPhase: (phase: string) => void;
  focused: boolean;
  onFocus: () => void;
  live: boolean;
  state?: StepState;
  percent?: number;
  pmState?: string | null;
  actions?: ReactNode;
}) {
  const tabs =
    pane === "shared"
      ? [
          { id: "all", label: "All" },
          ...["preflight", "clone", "release"]
            .filter((k) => steps.some((s) => s.kind === k))
            .map((k) => ({ id: k, label: k })),
        ]
      : phasesForProcess({
          id: pane,
          name: title,
          state: state || "pending",
          percent: percent || 0,
          steps,
        });
  const text = paneText(view, pane, phase);

  return (
    <section
      className={`panel deploy-proc-term${focused ? " is-focused" : ""}${state === "failed" ? " is-failed" : ""}`}
      onClick={onFocus}
    >
      <div className="deploy-proc-top">
        <div>
          <div className="deploy-proc-name">{title}</div>
          <div className={`deploy-proc-state is-${state || "pending"}`}>
            {state ? stateLabel(state) : "Server"}
            {pmState ? ` · pm2 ${pmState}` : ""}
          </div>
        </div>
        {typeof percent === "number" && (
          <div className="deploy-live-pct sm" aria-hidden="true">{percent}%</div>
        )}
      </div>
      {actions && <div className="deploy-proc-actions-wrap">{actions}</div>}
      {typeof percent === "number" && (
        <div
          className={`deploy-meter${state === "failed" ? " is-failed" : ""}`}
          role="progressbar"
          aria-label={title}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent}
        >
          <span style={{ width: `${percent}%` }} />
        </div>
      )}
      <div className="deploy-term-tabs deploy-term-embedded">
        <div className="deploy-term-tablist" role="tablist" aria-label={`${title} phases`}>
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={phase === t.id}
              className={`deploy-term-tab${phase === t.id ? " is-active" : ""}`}
              onClick={(e) => {
                e.stopPropagation();
                onPhase(t.id);
                onFocus();
              }}
            >
              {t.label}
            </button>
          ))}
        </div>
        <LogTerminal title={`${title}${phase !== "all" ? ` · ${phase}` : ""}`} text={text} live={live} />
      </div>
    </section>
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
  live,
}: {
  title: string;
  text: string;
  live: boolean;
}) {
  const ref = useRef<HTMLPreElement>(null);
  const stick = useRef(true);
  const count = text ? text.split("\n").filter((l) => l.trim()).length : 0;

  useEffect(() => {
    const el = ref.current;
    if (!el || !stick.current) return;
    el.scrollTop = el.scrollHeight;
  }, [text]);

  return (
    <TerminalFrame title={`${title} · ${count} ${count === 1 ? "line" : "lines"}`}>
      <pre
        ref={ref}
        className="term-log is-short"
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
    </TerminalFrame>
  );
}
