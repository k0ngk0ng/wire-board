import type { Event, Point, Objective } from "./types.ts";

export type EventCursor = {
  room: string;
  tick: number;
  lastID: number;
  at: number;
};
// Event IDs belong to one running game. A reconnect resumes the battlefield,
// not the historical event ring. Keep a small visual budget for large battles.
export function freshEvents(
  cursor: EventCursor | null,
  room: string,
  snapshot: { tick: number; events: Event[] },
  at: number,
): [EventCursor, Event[]] {
  const lastID = Math.max(0, ...snapshot.events.map((e) => e.id));
  const next = {
    room,
    tick: snapshot.tick,
    lastID: Math.max(lastID, cursor?.room === room ? cursor.lastID : 0),
    at,
  };
  if (
    !cursor ||
    cursor.room !== room ||
    at - cursor.at > 4000 ||
    snapshot.tick < cursor.tick
  ) {
    next.lastID = lastID;
    return [next, []];
  }
  return [next, snapshot.events.filter((e) => e.id > cursor.lastID).slice(-40)];
}
export function pointOnPath(path: Point[], distance: number): Point {
  if (!path.length) return { x: 0, y: 0 };
  let remaining = Number.isFinite(distance) ? Math.max(0, distance) : 0;
  for (let i = 1; i < path.length; i++) {
    const start = path[i - 1],
      end = path[i],
      length = Math.hypot(end.x - start.x, end.y - start.y);
    if (length > 0 && remaining <= length) {
      const p = remaining / length;
      return {
        x: start.x + (end.x - start.x) * p,
        y: start.y + (end.y - start.y) * p,
      };
    }
    remaining -= length;
  }
  return { ...path[path.length - 1] };
}
export function enemyPoint(
  path: Point[],
  previous:
    | (Point & { id: number | string; progress: number; route?: string })
    | undefined,
  current: Point & { id: number | string; progress: number; route?: string },
  fraction: number,
  reduced: boolean,
): Point {
  if (
    reduced ||
    !previous ||
    previous.id !== current.id ||
    previous.route !== current.route ||
    current.progress < previous.progress
  )
    return { x: current.x, y: current.y };
  const expected = pointOnPath(path, current.progress);
  // Saved enemies may predate route expansion or have switched to a shortcut.
  // Draw the authoritative point instead of sweeping across unrelated geometry.
  if (Math.hypot(expected.x - current.x, expected.y - current.y) > 2)
    return { x: current.x, y: current.y };
  const t = Math.min(1, Math.max(0, fraction));
  return pointOnPath(
    path,
    previous.progress + (current.progress - previous.progress) * t,
  );
}
type MovingObjective = Pick<
  Objective,
  "id" | "kind" | "status" | "route" | "progress" | "x" | "y"
>;
export function objectivePoint(
  path: Point[],
  previous: MovingObjective | undefined,
  current: MovingObjective,
  fraction: number,
  reduced: boolean,
  moving: boolean,
): Point {
  if (
    !moving ||
    current.kind !== "convoy" ||
    current.status !== "active" ||
    previous?.status !== "active"
  )
    return { x: current.x, y: current.y };
  return enemyPoint(path, previous, current, fraction, reduced);
}
export function pickSound(
  events: Event[],
  at: number,
  lastFireAt: number,
): string | null {
  const priority: [string, string][] = [
    ["win", "victory"],
    ["lose", "defeat"],
    ["base_hit", "leak"],
    ["expand", "expand"],
    ["objective", "objective"],
    ["boon", "objective"],
    ["boss_phase", "event"],
    ["shortcut", "event"],
    ["wave", "wave"],
    ["upgrade", "upgrade"],
    ["build", "build"],
    ["sell", "sell"],
  ];
  for (const [type, sound] of priority)
    if (events.some((e) => e.type === type)) return sound;
  if (at - lastFireAt < 700) return null;
  const shot = events.find((e) => e.type === "shot");
  return shot &&
    ["arrow", "cannon", "frost", "arcane"].includes(shot.kind ?? "")
    ? shot.kind!
    : null;
}
export function assetURL(base: string, file: string): string {
  return `${base.replace(/\/$/, "")}/starfort/v1/${file}`;
}

// Large mobile sprites may be drawn beside their original anchor at an edge.
// Only the artwork moves: range, targeting, selection, and server pads do not.
export function spritePlacement(
  anchor: Point,
  width: number,
  height: number,
  bounds: { width: number; height: number; top?: number },
): Point {
  const inset = 4;
  return {
    x: Math.max(
      width / 2 + inset,
      Math.min(bounds.width - width / 2 - inset, anchor.x),
    ),
    y: Math.max(
      (bounds.top ?? 0) + height + inset,
      Math.min(bounds.height - inset, anchor.y),
    ),
  };
}

// Add camera headroom rather than moving a tower into a neighboring pad.
// The renderer and HTML hit targets apply the same world-space offset.
export function battlefieldTopInset(
  viewportWidth: number,
  mapWidth: number,
  minPadY: number,
): number {
  if (viewportWidth <= 0 || mapWidth <= 0) return 0;
  const towerSize = Math.max(66, 42 / (viewportWidth / mapWidth));
  return Math.max(0, Math.ceil((towerSize * 224) / 192 + 4 - (minPadY + 14)));
}
