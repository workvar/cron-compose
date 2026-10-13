"use client";

import { useState } from "react";
import type { ProjectBlock } from "@/lib/project-blocks";
import type { WaveFailurePolicy } from "@/lib/deploy-spec";
import {
  addEmptyWave,
  defaultFlow,
  flowProblems,
  moveTask,
  phaseLabel,
  removeWave,
  type FlowPhase,
  type FlowWave,
} from "@/lib/deploy-flow";

type Props = {
  blocks: ProjectBlock[];
  flow: FlowWave[];
  onFlow: (flow: FlowWave[]) => void;
  onWaveFailure: WaveFailurePolicy;
  onOnWaveFailure: (policy: WaveFailurePolicy) => void;
};

/**
 * Column drag-and-drop deploy flow. Each chip is one task (install/build/… for
 * one process). Same column runs in parallel; later columns wait. Empty columns
 * must be filled or deleted before save.
 */
export function DeployOrderBoard({
  blocks,
  flow,
  onFlow,
  onWaveFailure,
  onOnWaveFailure,
}: Props) {
  const [dragId, setDragId] = useState<string | null>(null);
  const [over, setOver] = useState<{ waveId: string; beforeId: string | null } | null>(null);

  if (blocks.length < 1) return null;

  const problems = flowProblems(flow);

  function onDrop(waveId: string, beforeId: string | null) {
    if (!dragId) return;
    onFlow(moveTask(flow, dragId, waveId, beforeId));
    setDragId(null);
    setOver(null);
  }

  return (
    <div className="deploy-order">
      <div className="cluster" style={{ justifyContent: "space-between", alignItems: "baseline", gap: 12 }}>
        <div>
          <div className="card-title">Deploy flow</div>
          <p className="field-hint" style={{ margin: "4px 0 0" }}>
            Drag tasks into columns (waves). Tasks in one wave run together; later waves wait.
            Mix install / build / cleanup across apps — e.g. build A after install B.
            Release activates between cleanup and pre-start automatically.
          </p>
        </div>
        <div className="cluster" style={{ gap: 6, flexWrap: "wrap" }}>
          <button
            type="button"
            className="button ghost sm"
            onClick={() => onFlow(defaultFlow(blocks))}
          >
            Reset default
          </button>
          <button
            type="button"
            className="button ghost sm"
            onClick={() => onFlow(addEmptyWave(flow))}
          >
            + Wave
          </button>
        </div>
      </div>

      <div className="deploy-order-failure" role="group" aria-label="When a wave fails">
        <div className="deploy-order-failure-label">If a wave fails</div>
        <div className="deploy-order-failure-opts">
          <label className={`deploy-order-failure-opt${onWaveFailure === "stop" ? " is-on" : ""}`}>
            <input
              type="radio"
              name="on-wave-failure"
              checked={onWaveFailure === "stop"}
              onChange={() => onOnWaveFailure("stop")}
            />
            <span>
              <strong>Stop the deploy</strong>
              <span className="subtle">End the run. Later waves do not start.</span>
            </span>
          </label>
          <label className={`deploy-order-failure-opt${onWaveFailure === "continue" ? " is-on" : ""}`}>
            <input
              type="radio"
              name="on-wave-failure"
              checked={onWaveFailure === "continue"}
              onChange={() => onOnWaveFailure("continue")}
            />
            <span>
              <strong>Continue to the next wave</strong>
              <span className="subtle">
                Skip the rest of this wave; later waves still run. The deploy is still marked failed
                if any wave failed.
              </span>
            </span>
          </label>
        </div>
      </div>

      {problems.length > 0 && (
        <p className="form-error" style={{ marginTop: 12 }}>{problems[0]}</p>
      )}

      <div className="deploy-order-columns" role="list">
        {flow.map((wave, i) => (
          <div
            key={wave.id}
            role="listitem"
            className={`deploy-order-wave deploy-order-column${
              over?.waveId === wave.id && !over.beforeId ? " is-over" : ""
            }${wave.tasks.length === 0 ? " is-empty" : ""}`}
            onDragOver={(e) => {
              e.preventDefault();
              setOver({ waveId: wave.id, beforeId: null });
            }}
            onDrop={(e) => {
              e.preventDefault();
              onDrop(wave.id, null);
            }}
          >
            <div className="deploy-order-wave-label">
              <span>
                Wave {i + 1}
                <span className="subtle">
                  {wave.tasks.length === 0
                    ? " · empty"
                    : wave.tasks.length === 1
                      ? " · 1 task"
                      : ` · ${wave.tasks.length} together`}
                </span>
              </span>
              {wave.tasks.length === 0 && (
                <button
                  type="button"
                  className="button ghost sm"
                  onClick={() => onFlow(removeWave(flow, wave.id))}
                  title="Delete empty wave"
                >
                  Delete
                </button>
              )}
            </div>
            <div className="deploy-order-chips">
              {wave.tasks.map((task) => {
                const block = blocks.find((b) => b.id === task.blockId);
                if (!block) return null;
                return (
                  <button
                    key={task.id}
                    type="button"
                    className={`deploy-order-chip${dragId === task.id ? " is-dragging" : ""}${
                      over?.beforeId === task.id ? " is-before" : ""
                    }`}
                    draggable
                    onDragStart={(e) => {
                      setDragId(task.id);
                      e.dataTransfer.effectAllowed = "move";
                      e.dataTransfer.setData("text/plain", task.id);
                    }}
                    onDragEnd={() => {
                      setDragId(null);
                      setOver(null);
                    }}
                    onDragOver={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      setOver({ waveId: wave.id, beforeId: task.id });
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      onDrop(wave.id, task.id);
                    }}
                  >
                    <span className="deploy-order-handle" aria-hidden="true">⋮⋮</span>
                    <span className="deploy-order-chip-body">
                      <span className="deploy-order-chip-name">{block.name || "Process"}</span>
                      <span className={`deploy-order-chip-phase phase-${task.phase as FlowPhase}`}>
                        {phaseLabel(task.phase)}
                      </span>
                    </span>
                  </button>
                );
              })}
              {wave.tasks.length === 0 && (
                <span className="subtle" style={{ fontSize: 13 }}>Drop a task here</span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
