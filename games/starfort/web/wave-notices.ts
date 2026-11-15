import type { Geology, MapData, View } from "./types.ts";

// The authoritative spawn plan distributes enemies round-robin over the
// snapshot's active entries. The boss is the final planned spawn (battle.go).
export function bossEntry(map: MapData, wave: Pick<View["waveInfo"], "boss" | "total">) {
  if (!wave.boss || wave.total < 1) return undefined;
  const entries = map.routes?.filter((route) => route.active) ?? [{ id: "main", label: "主路" }];
  return entries.length ? entries[(wave.total - 1) % entries.length] : undefined;
}

export function activeDisasterLabel(geology: Geology | undefined, elapsedMS: number): string | null {
  if (!geology?.triggered || geology.rewarded || geology.kind === "none") return null;
  const active = geology.zones?.some((zone) =>
    !zone.skipped && zone.createdMS <= elapsedMS &&
    (!zone.expiresMS || elapsedMS < zone.expiresMS) &&
    (zone.kind !== "typhoon" || zone.activated),
  );
  return active ? ({ earthquake: "地震", flood: "洪水", typhoon: "台风" } as const)[geology.kind] : null;
}

export function taskPromptKey(room: string, match: string | number | undefined, wave: number, task: string) {
  return JSON.stringify([room, match, wave, task]);
}

export function shouldPromptTask(state: {
  phase: string; spectating?: boolean; bossOpen?: boolean; worldChoice?: boolean;
  rewardOpen?: boolean; placing?: boolean;
}) {
  return state.phase === "prepare" && !state.spectating && !state.bossOpen &&
    !state.worldChoice && !state.rewardOpen && !state.placing;
}

type Rect = { left: number; top: number; right: number; bottom: number };
type Point = { x: number; y: number };
const overlaps = (a: Rect, b: Rect) => a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
function crosses(a: Point, b: Point, rect: Rect, padding: number) {
  let lo = 0, hi = 1;
  for (const [start, delta, min, max] of [
    [a.x, b.x - a.x, rect.left - padding, rect.right + padding],
    [a.y, b.y - a.y, rect.top - padding, rect.bottom + padding],
  ]) {
    if (Math.abs(delta) < 1e-8) { if (start < min || start > max) return false; continue; }
    const from = (min - start) / delta, to = (max - start) / delta;
    lo = Math.max(lo, Math.min(from, to)); hi = Math.min(hi, Math.max(from, to));
    if (lo > hi) return false;
  }
  return true;
}

// Position only the small overlay. No padding, camera changes or layout rows.
// Prefer a grass gap on the map's left edge while keeping controls accessible.
export function noticePosition(
  viewport: { width: number; height: number }, box: { width: number; height: number },
  avoid: Rect[], roads: Point[][], roadPadding: number,
) {
  const left = 8, width = Math.min(box.width, Math.max(1, viewport.width - 16));
  const bottom = Math.max(8, viewport.height - box.height - 8);
  let best = { left, top: 8 }, bestScore = Infinity;
  for (let top = 8; top <= bottom; top += 4) {
    const rect = { left, top, right: left + width, bottom: top + box.height };
    let score = avoid.filter((item) => overlaps(rect, item)).length * 10000;
    for (const path of roads) for (let i = 1; i < path.length; i++)
      if (crosses(path[i - 1], path[i], rect, roadPadding)) score += 1;
    if (score < bestScore) { best = { left, top }; bestScore = score; }
    if (score === 0) break;
  }
  return best;
}
