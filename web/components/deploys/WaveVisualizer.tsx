"use client";

import type { WaveLive, WaveTaskLive } from "@/lib/deploy-waves";
import type { StepState } from "@/lib/deploy-progress";

type Props = {
  waves: WaveLive[];
  activeIndex: number;
  selectedPane: string;
  onSelectPane: (pane: string) => void;
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

function TaskCard({
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
      className={`deploy-wave-task is-${task.state}${selected ? " is-selected" : ""}`}
      onClick={onSelect}
      aria-pressed={selected}
    >
      <div className="deploy-wave-task-top">
        <Mark state={task.state} />
        <div className="deploy-wave-task-copy">
          <strong>{task.app}</strong>
          <span>{task.phase.replace(/_/g, " ")}</span>
        </div>
        <span className="deploy-wave-task-pct">{task.percent}%</span>
      </div>
      <div
        className={`deploy-meter${task.state === "failed" ? " is-failed" : ""}`}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={task.percent}
        aria-label={task.label}
      >
        <span style={{ width: `${task.percent}%` }} />
      </div>
      <div className={`deploy-wave-task-state is-${task.state}`}>{stateLabel(task.state)}</div>
    </button>
  );
}

export function WaveVisualizer({ waves, activeIndex, selectedPane, onSelectPane }: Props) {
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

      <ol className="deploy-wave-strip" aria-label="Wave progress">
        {waves.map((w, i) => (
          <li key={w.index} className={`deploy-wave-pill is-${w.state}${i === activeIndex ? " is-current" : ""}`}>
            <Mark state={w.state} />
            <span>W{w.index}</span>
            <span className="deploy-wave-pill-pct">{w.percent}%</span>
          </li>
        ))}
      </ol>

      <div className="deploy-wave-tasks" role="list">
        {focus.tasks.map((t) => (
          <TaskCard
            key={t.id}
            task={t}
            selected={selectedPane === t.pane}
            onSelect={() => onSelectPane(t.pane)}
          />
        ))}
      </div>
    </section>
  );
}
