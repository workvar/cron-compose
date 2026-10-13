"use client";

import type { WaveLive, WaveTaskLive } from "@/lib/deploy-waves";
import type { StepState } from "@/lib/deploy-progress";

type Props = {
  waves: WaveLive[];
  activeIndex: number;
  selectedPane: string;
  selectedPhase?: string;
  onSelectTask: (pane: string, phase: string) => void;
};

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

function Mark({ state }: { state: StepState }) {
  return (
    <span className={`deploy-mark is-${state}`} aria-hidden="true">
      {state === "done" ? "✓" : state === "failed" ? "!" : state === "skipped" ? "–" : ""}
    </span>
  );
}

function TaskNode({
  task,
  selected,
  onSelect,
}: {
  task: WaveTaskLive;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      className={`deploy-flow-node is-${task.state}${selected ? " is-selected" : ""}`}
      onClick={onSelect}
      aria-pressed={selected}
      title={task.label}
    >
      <Mark state={task.state} />
      <span className="deploy-flow-node-body">
        <span className="deploy-flow-node-app">{task.app}</span>
        <span className="deploy-flow-node-phase">{task.phase.replace(/_/g, " ")}</span>
        <span className={`deploy-flow-node-state is-${task.state}`}>{stateLabel(task.state)}</span>
      </span>
      <span className="deploy-flow-node-pct">{task.percent}%</span>
    </button>
  );
}

function Arrow() {
  return (
    <span className="deploy-flow-arrow" aria-hidden="true">
      <svg width="28" height="12" viewBox="0 0 28 12" fill="none">
        <path
          d="M0 6h22M18 1.5L24.5 6 18 10.5"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </span>
  );
}

export function WaveVisualizer({
  waves,
  activeIndex,
  selectedPane,
  selectedPhase,
  onSelectTask,
}: Props) {
  if (waves.length === 0) return null;
  const focus = waves[Math.min(Math.max(activeIndex, 0), waves.length - 1)];

  return (
    <section className="panel deploy-wave-board" aria-label="Deploy waves">
      <div className="card-head">
        <div>
          <div className="card-title">Waves</div>
          <div className="deploy-live-headline">
            Wave {focus.index} of {focus.total}
            {focus.tasks.length > 1 ? ` · ${focus.tasks.length} together` : ""}
          </div>
        </div>
        <div className={`deploy-live-pct sm is-${focus.state}`}>{focus.percent}%</div>
      </div>

      <div className="deploy-flow" role="list" aria-label="Task flow">
        {waves.map((w, i) => (
          <div key={w.index} className="deploy-flow-row" role="listitem">
            {i > 0 && <Arrow />}
            <div
              className={`deploy-flow-wave is-${w.state}${i === activeIndex ? " is-current" : ""}`}
            >
              <div className="deploy-flow-wave-head">
                <Mark state={w.state} />
                <span>W{w.index}</span>
                <span className="deploy-flow-wave-pct">{w.percent}%</span>
              </div>
              <div className="deploy-flow-wave-tasks">
                {w.tasks.map((t, ti) => (
                  <div key={t.id} className="deploy-flow-wave-task-wrap">
                    {ti > 0 && (
                      <span className="deploy-flow-parallel" aria-hidden="true">
                        +
                      </span>
                    )}
                    <TaskNode
                      task={t}
                      selected={selectedPane === t.pane && selectedPhase === t.phase}
                      onSelect={() => onSelectTask(t.pane, t.phase)}
                    />
                  </div>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
