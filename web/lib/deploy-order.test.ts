import assert from "node:assert/strict";
import type { ProjectBlock } from "./project-blocks.ts";
import {
  allTogetherWaves,
  blocksWithWaves,
  moveBlockToWave,
  oneByOneWaves,
  wavesFromBlocks,
} from "./deploy-order.ts";

function block(id: string, name: string, startGroup = 0): ProjectBlock {
  return {
    id,
    name,
    root: name.toLowerCase(),
    language: "node",
    install: "npm i",
    run: "npm start",
    cleanup: "",
    preStart: "",
    port: "",
    processManager: "pm2",
    autoDetect: false,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
    startGroup,
  };
}

const blocks = [block("a", "Proxy"), block("b", "Backend", 1), block("c", "Frontend", 1)];

{
  const waves = wavesFromBlocks(blocks);
  assert.deepEqual(waves, [
    { group: 0, blockIds: ["a"] },
    { group: 1, blockIds: ["b", "c"] },
  ]);
}

{
  const next = moveBlockToWave(wavesFromBlocks(blocks), "c", 0);
  assert.deepEqual(next, [
    { group: 0, blockIds: ["a", "c"] },
    { group: 1, blockIds: ["b"] },
  ]);
  const applied = blocksWithWaves(blocks, next);
  assert.equal(applied.find((b) => b.id === "c")?.startGroup, 0);
  assert.equal(applied.find((b) => b.id === "b")?.startGroup, 1);
}

{
  assert.deepEqual(allTogetherWaves(blocks), [{ group: 0, blockIds: ["a", "b", "c"] }]);
  assert.deepEqual(oneByOneWaves(blocks), [
    { group: 0, blockIds: ["a"] },
    { group: 1, blockIds: ["b"] },
    { group: 2, blockIds: ["c"] },
  ]);
}

console.log("deploy-order: ok");
