"use client";

import { useState } from "react";
import type { ProjectBlock } from "@/lib/project-blocks";
import type { WaveFailurePolicy } from "@/lib/deploy-spec";
import {
  addEmptyWave,
  allTogetherWaves,
  blocksWithWaves,
  moveBlockToWave,
  oneByOneWaves,
  wavesFromBlocks,
  type DeployWave,
} from "@/lib/deploy-order";

type Props = {
  blocks: ProjectBlock[];
  onBlocks: (blocks: ProjectBlock[]) => void;
  onWaveFailure: WaveFailurePolicy;
  onOnWaveFailure: (policy: WaveFailurePolicy) => void;
};

/**
 * Drag-and-drop start waves. Apps in one wave run pre-start/start/health
 * together; wave 2 waits for wave 1. Install/build stay parallel for every app.
 */
export function DeployOrderBoard({ blocks, onBlocks, onWaveFailure, onOnWaveFailure }: Props) {
  const waves = wavesFromBlocks(blocks);
  const [dragId, setDragId] = useState<string | null>(null);
  const [over, setOver] = useState<{ group: number; beforeId: string | null } | null>(null);

  if (blocks.length < 2) return null;

  function apply(next: DeployWave[]) {
    onBlocks(blocksWithWaves(blocks, next));
  }

  function onDrop(group: number, beforeId: string | null) {
    if (!dragId) return;
    apply(moveBlockToWave(waves, dragId, group, beforeId));
    setDragId(null);
    setOver(null);
  }

  return (
    <div className="deploy-order">
      <div className="cluster" style={{ justifyContent: "space-between", alignItems: "baseline", gap: 12 }}>
        <div>
          <div className="card-title">Deploy order</div>
          <p className="field-hint" style={{ margin: "4px 0 0" }}>
            Drag processes into waves. Same wave starts together; later waves wait.
            Builds still run in parallel for every app.
          </p>
        </div>
        <div className="cluster" style={{ gap: 6, flexWrap: "wrap" }}>
          <button
            type="button"
            className="button ghost sm"
            onClick={() => apply(allTogetherWaves(blocks))}
          >
            All together
          </button>
          <button
            type="button"
            className="button ghost sm"
            onClick={() => apply(oneByOneWaves(blocks))}
          >
            One by one
          </button>
          <button
            type="button"
            className="button ghost sm"
            onClick={() => apply(addEmptyWave(waves))}
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
                if any wave failed. Within a wave, sibling processes still finish together.
              </span>
            </span>
          </label>
        </div>
      </div>

      <div className="deploy-order-waves">
        {waves.map((wave, i) => (
          <div
            key={wave.group}
            className={`deploy-order-wave${over?.group === wave.group && !over.beforeId ? " is-over" : ""}`}
            onDragOver={(e) => {
              e.preventDefault();
              setOver({ group: wave.group, beforeId: null });
            }}
            onDrop={(e) => {
              e.preventDefault();
              onDrop(wave.group, null);
            }}
          >
            <div className="deploy-order-wave-label">
              Wave {i + 1}
              <span className="subtle">
                {wave.blockIds.length > 1 ? " · together" : wave.blockIds.length === 1 ? " · alone" : " · empty"}
              </span>
            </div>
            <div className="deploy-order-chips">
              {wave.blockIds.map((id) => {
                const block = blocks.find((b) => b.id === id);
                if (!block) return null;
                return (
                  <button
                    key={id}
                    type="button"
                    className={`deploy-order-chip${dragId === id ? " is-dragging" : ""}${
                      over?.beforeId === id ? " is-before" : ""
                    }`}
                    draggable
                    onDragStart={(e) => {
                      setDragId(id);
                      e.dataTransfer.effectAllowed = "move";
                      e.dataTransfer.setData("text/plain", id);
                    }}
                    onDragEnd={() => {
                      setDragId(null);
                      setOver(null);
                    }}
                    onDragOver={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      setOver({ group: wave.group, beforeId: id });
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      onDrop(wave.group, id);
                    }}
                  >
                    <span className="deploy-order-handle" aria-hidden="true">⋮⋮</span>
                    {block.name || "Process"}
                  </button>
                );
              })}
              {wave.blockIds.length === 0 && (
                <span className="subtle" style={{ fontSize: 13 }}>Drop a process here</span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
