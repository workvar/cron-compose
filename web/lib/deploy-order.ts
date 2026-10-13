import type { ProjectBlock } from "./project-blocks";

/** One start wave: apps in the same group run pre-start/start/health together. */
export type DeployWave = {
  group: number;
  blockIds: string[];
};

/** Build ordered waves from blocks. Empty groups are dropped; gaps are renumbered. */
export function wavesFromBlocks(blocks: ProjectBlock[]): DeployWave[] {
  const buckets = new Map<number, string[]>();
  for (const b of blocks) {
    const g = Number.isFinite(b.startGroup) && b.startGroup > 0 ? Math.floor(b.startGroup) : 0;
    const list = buckets.get(g) ?? [];
    list.push(b.id);
    buckets.set(g, list);
  }
  return [...buckets.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([, blockIds], i) => ({ group: i, blockIds }));
}

/** Apply wave membership back onto blocks (contiguous startGroup 0..n-1). */
export function blocksWithWaves(blocks: ProjectBlock[], waves: DeployWave[]): ProjectBlock[] {
  const byId = new Map(blocks.map((b) => [b.id, b]));
  const seen = new Set<string>();
  const out: ProjectBlock[] = [];
  waves.forEach((wave, i) => {
    for (const id of wave.blockIds) {
      const b = byId.get(id);
      if (!b || seen.has(id)) continue;
      seen.add(id);
      out.push({ ...b, startGroup: i });
    }
  });
  // Keep any block the board dropped (should not happen) at the end.
  for (const b of blocks) {
    if (!seen.has(b.id)) out.push({ ...b, startGroup: Math.max(0, waves.length) });
  }
  return out;
}

export function moveBlockToWave(
  waves: DeployWave[],
  blockId: string,
  toGroup: number,
  beforeBlockId?: string | null,
): DeployWave[] {
  const cleaned = waves.map((w) => ({
    ...w,
    blockIds: w.blockIds.filter((id) => id !== blockId),
  }));
  while (cleaned.length <= toGroup) {
    cleaned.push({ group: cleaned.length, blockIds: [] });
  }
  const target = cleaned[toGroup];
  const ids = [...target.blockIds];
  const at = beforeBlockId ? ids.indexOf(beforeBlockId) : -1;
  if (at >= 0) ids.splice(at, 0, blockId);
  else ids.push(blockId);
  cleaned[toGroup] = { ...target, blockIds: ids };
  return cleaned
    .filter((w) => w.blockIds.length > 0)
    .map((w, i) => ({ group: i, blockIds: w.blockIds }));
}

export function addEmptyWave(waves: DeployWave[]): DeployWave[] {
  return [...waves, { group: waves.length, blockIds: [] }];
}

/** Preset: every process in one wave (current default). */
export function allTogetherWaves(blocks: ProjectBlock[]): DeployWave[] {
  return [{ group: 0, blockIds: blocks.map((b) => b.id) }];
}

/** Preset: each process its own wave, in current list order. */
export function oneByOneWaves(blocks: ProjectBlock[]): DeployWave[] {
  return blocks.map((b, i) => ({ group: i, blockIds: [b.id] }));
}
