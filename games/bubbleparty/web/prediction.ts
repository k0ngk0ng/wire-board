import type { Move } from "./input";
import type { Player, View } from "./types";

import catalog from "../data/classic-2002.json" with { type: "json" };
type Point = { x: number; y: number };
type TerrainView = Pick<
  View,
  "mapId" | "nativeObjects" | "boxMotions" | "serverAt"
>;
const nativeMaps = new Map(catalog.maps.map((m) => [m.id, m]));
const nativeTiles = new Map(catalog.tiles.map((t) => [t.id, t]));
const passesBoxes = (p: Player) =>
  p.mount === "airship" || p.mount === "airship-broken";

// Predict plain travel only. Special terrain and active pushed bodies use
// confirmed server positions; guessing their state would invent gate/belt moves.
export function authoritativeTerrain(
  view: Partial<TerrainView>,
  point: Point,
): boolean {
  if (
    (view.boxMotions ?? []).some(
      (m) =>
        m.endAt > (view.serverAt ?? 0) &&
        Math.abs(point.x - (m.fromX + m.x) / 2) < 2.5 &&
        Math.abs(point.y - (m.fromY + m.y) / 2) < 2.5,
    )
  )
    return true;
  const map = nativeMaps.get(view.mapId ?? "");
  if (!map) return false;
  const ox = Math.floor((15 - map.width) / 2),
    oy = Math.floor((13 - map.height) / 2);
  for (let y = Math.floor(point.y) - 2; y <= Math.ceil(point.y) + 2; y++)
    for (let x = Math.floor(point.x) - 2; x <= Math.ceil(point.x) + 2; x++) {
      if (x < 0 || x >= 15 || y < 0 || y >= 13) continue;
      const ground = nativeTiles.get(map.ground[y - oy]?.[x - ox]);
      if (ground?.category === "ground" && ground.tileInfo.fieldsRaw[3] === 2)
        return true;
      const id =
        view.nativeObjects !== undefined
          ? view.nativeObjects[y * 15 + x]
          : map.objects[y - oy]?.[x - ox];
      const object = nativeTiles.get(id ?? -1);
      if (
        object?.category === "passable" &&
        object.tileInfo.fieldsRaw.slice(1, 5).some((v) => v === 0)
      )
        return true;
    }
  return false;
}
const radius = 0.28;
const horizon = 250;
const epsilon = 0.000001;
const position = (p: Player): Point => ({ x: p.px ?? p.x, y: p.py ?? p.y });

// Display geometry only. No bombs, pickups, trapped state or results are created.
// Sweeping the body against the visible grid also constrains reconciliation.
function axis(
  from: Point,
  target: number,
  horizontal: boolean,
  cells: readonly number[],
  passBoxes = false,
) {
  const old = horizontal ? from.x : from.y;
  const across = horizontal ? from.y : from.x;
  const positive = target > old;
  if (Math.abs(target - old) < epsilon) return old;
  const minX = Math.max(
    -1,
    Math.floor(Math.min(from.x, horizontal ? target : from.x)) - 1,
  );
  const maxX = Math.min(
    15,
    Math.ceil(Math.max(from.x, horizontal ? target : from.x)) + 1,
  );
  const minY = Math.max(
    -1,
    Math.floor(Math.min(from.y, horizontal ? from.y : target)) - 1,
  );
  const maxY = Math.min(
    13,
    Math.ceil(Math.max(from.y, horizontal ? from.y : target)) + 1,
  );
  for (let y = minY; y <= maxY; y++)
    for (let x = minX; x <= maxX; x++) {
      const cell = x < 0 || x >= 15 || y < 0 || y >= 13 ? 1 : cells[y * 15 + x];
      if (cell !== 1 && cell !== 2 && cell !== 3 && cell !== 4) continue;
      if (passBoxes && (cell === 2 || cell === 4)) continue;
      const center = horizontal ? x : y,
        cross = horizontal ? y : x;
      if (Math.abs(across - cross) >= 0.5 + radius - epsilon) continue;
      const low = center - 0.5 - radius,
        high = center + 0.5 + radius;
      if (old > low + epsilon && old < high - epsilon) {
        const sameCell = Math.round(from.x) === x && Math.round(from.y) === y;
        if (
          cell === 3 &&
          (sameCell ||
            (positive && old >= center) ||
            (!positive && old <= center))
        )
          continue;
        if (target > low && target < high) target = old;
      } else if (positive && old <= low + epsilon && target > low) target = low;
      else if (!positive && old >= high - epsilon && target < high)
        target = high;
    }
  return target;
}

function constrain(
  from: Point,
  target: Point,
  cells: readonly number[],
  passBoxes = false,
): Point {
  const x = axis(from, target.x, true, cells, passBoxes);
  return { x, y: axis({ x, y: from.y }, target.y, false, cells, passBoxes) };
}

export function predictPlayer(
  from: Point,
  player: Player,
  move: Move,
  cells: readonly number[],
  ms: number,
  terrain: Partial<TerrainView> = {},
): Point {
  const p = { ...from };
  if (!player.alive || player.trappedUntil > 0 || move === "none") return p;
  const passBoxes = passesBoxes(player);
  const dx = move === "left" ? -1 : move === "right" ? 1 : 0;
  const dy = move === "up" ? -1 : move === "down" ? 1 : 0;
  let remaining = Math.max(0, Math.min(horizon, ms));
  while (remaining > 0) {
    if (authoritativeTerrain(terrain, p)) break;
    const step = Math.min(20, remaining);
    remaining -= step;
    let distance = step / Math.max(70, player.speedMs ?? 150);
    // A turn first aligns to the current row/column, without cutting a corner.
    if (dx && Math.abs(p.y - Math.round(p.y)) > epsilon) {
      const center = Math.round(p.y);
      const y = axis(
        p,
        p.y +
          Math.sign(center - p.y) * Math.min(distance, Math.abs(center - p.y)),
        false,
        cells,
        passBoxes,
      );
      distance -= Math.abs(y - p.y);
      p.y = y;
      if (Math.abs(p.y - center) > epsilon) distance = 0;
    }
    if (dy && Math.abs(p.x - Math.round(p.x)) > epsilon) {
      const center = Math.round(p.x);
      const x = axis(
        p,
        p.x +
          Math.sign(center - p.x) * Math.min(distance, Math.abs(center - p.x)),
        true,
        cells,
        passBoxes,
      );
      distance -= Math.abs(x - p.x);
      p.x = x;
      if (Math.abs(p.x - center) > epsilon) distance = 0;
    }
    if (dx) p.x = axis(p, p.x + dx * distance, true, cells, passBoxes);
    if (dy) p.y = axis(p, p.y + dy * distance, false, cells, passBoxes);
  }
  return p;
}

// Rebase to each authoritative physics sample; cosmetic error decays instead
// of accumulating. Missing snapshots cap local integration at 250ms. Transport,
// control and life-state changes immediately discard all old display history.
export class PlayerMotion {
  private scope = "";
  private stamp = -Infinity;
  private received = 0;
  private at = 0;
  private active = false;
  private life = "";
  private policy = "authority";
  mode() {
    return this.policy;
  }
  private authority?: Point;
  private local?: Point;
  private shown?: Point;
  private correction: Point = { x: 0, y: 0 };
  frame(
    view: View,
    seat: number,
    move: Move,
    now: number,
    scope: string,
    enabled: boolean,
  ): Point {
    const player = view.players[seat],
      base = position(player);
    const active =
      enabled &&
      (view.rulesVersion ?? 0) >= 2 &&
      view.phase === "playing" &&
      player.alive &&
      !player.trappedUntil;
    const life = `${player.mount ?? ""}:${player.character ?? ""}:${player.alive}:${player.trappedUntil}`;
    const authorityOnly =
      active &&
      (view.rulesVersion ?? 0) >= 3 &&
      authoritativeTerrain(view, base);
    const policy = !active
      ? "authority"
      : authorityOnly
        ? "terrain-authority"
        : "predicted";
    if (
      !this.local ||
      life !== this.life ||
      policy !== this.policy ||
      scope !== this.scope ||
      active !== this.active ||
      !active
    ) {
      this.life = life;
      this.policy = policy;
      this.scope = scope;
      this.stamp = view.serverAt;
      this.received = now;
      this.at = now;
      this.active = active;
      this.authority = base;
      this.local = base;
      this.shown = base;
      this.correction = { x: 0, y: 0 };
      return { ...base };
    }
    if (authorityOnly) {
      const elapsed = Math.max(0, now - this.at);
      this.at = now;
      const previous = this.shown ?? base;
      const blend = 1 - Math.exp(-Math.min(100, elapsed) / 35);
      this.shown =
        Math.hypot(previous.x - base.x, previous.y - base.y) > 1.5
          ? base
          : {
              x: previous.x + (base.x - previous.x) * blend,
              y: previous.y + (base.y - previous.y) * blend,
            };
      this.authority = base;
      this.local = base;
      this.stamp = view.serverAt;
      return { ...this.shown };
    }
    if (
      this.stamp !== view.serverAt ||
      this.authority?.x !== base.x ||
      this.authority?.y !== base.y
    ) {
      const old = this.shown!;
      this.correction =
        Math.hypot(old.x - base.x, old.y - base.y) < 1.5
          ? { x: old.x - base.x, y: old.y - base.y }
          : { x: 0, y: 0 };
      this.local = base;
      this.authority = base;
      this.stamp = view.serverAt;
      this.received = now;
    }
    const elapsed = Math.max(0, now - this.at);
    const step = Math.min(
      40,
      elapsed,
      Math.max(0, this.received + horizon - this.at),
    );
    this.at = now;
    this.local = predictPlayer(
      this.local,
      player,
      move,
      view.cells,
      step,
      view,
    );
    const blend = Math.exp(-elapsed / 70);
    this.correction.x *= blend;
    this.correction.y *= blend;
    this.shown = constrain(
      this.local,
      {
        x: this.local.x + this.correction.x,
        y: this.local.y + this.correction.y,
      },
      view.cells,
      passesBoxes(player),
    );
    return { ...this.shown };
  }
}
