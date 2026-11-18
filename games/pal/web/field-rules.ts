/*!
 * SPDX-License-Identifier: GPL-3.0-only
 * Copyright (c) 2009-2011, Wei Mingzhi <whistler_wmz@users.sf.net>.
 * Copyright (c) 2011-2026, SDLPAL development team.
 * Portions based on PALx Project by palxex.
 * Copyright (c) 2006, Pal Lockheart <palxex@gmail.com>.
 * See games/pal/native/upstream/LICENSE for the GNU GPL version 3.
 */
// Movement, gestures and covering-tile ordering ported from pinned SDLPAL
// scene.c. The server supplies bounded original collision answers and public
// actors; scripts, NPC decisions, resources and combat stay authoritative.
export type FieldActor = [
  number,
  number,
  number,
  number,
  number,
  number,
  number,
  number,
];
export type FieldState = {
  frame: number;
  anchor: [number, number];
  camera: [number, number];
  offset: [number, number];
  layer: number;
  direction: number;
  step: number;
  party: number;
  followers: number;
  radius: number;
  trail: [number, number, number][];
  checks: [number, number][];
  cells: string;
  actors: FieldActor[];
};
export type FieldGeometry = {
  sprite: (
    asset: number,
    frame: number,
  ) => { width: number; height: number } | undefined;
  height: (x: number, y: number, half: number, layer: number) => number;
};
export type FieldDraw = {
  kind: number;
  a: number;
  b: number;
  c: number;
  d: number;
  x: number;
  baseY: number;
  sortY: number;
};
const grids = new Map<string, Uint8Array>();
export function fieldCell(state: FieldState, x: number, y: number): number {
  let cells = grids.get(state.cells);
  if (!cells) {
    cells = Uint8Array.from(atob(state.cells), (c) => c.charCodeAt(0));
    if (grids.size >= 64) grids.delete(grids.keys().next().value!);
    grids.set(state.cells, cells);
  }
  const dx = (x - state.anchor[0]) / 16,
    dy = (y - state.anchor[1]) / 8;
  const u = (dx + dy) / 2,
    v = (dy - dx) / 2,
    r = state.radius;
  if (
    !Number.isInteger(u) ||
    !Number.isInteger(v) ||
    Math.abs(u) > r ||
    Math.abs(v) > r
  ) {
    for (let i = 0; i < (state.checks?.length ?? 0); i++) {
      const px = (x - state.checks[i][0]) / 16,
        py = (y - state.checks[i][1]) / 8;
      if (
        Number.isInteger(px) &&
        Number.isInteger(py) &&
        Math.abs(px) <= 1 &&
        Math.abs(py) <= 1
      )
        return cells[289 + i * 9 + (py + 1) * 3 + px + 1] ?? 3;
    }
    return 3;
  }
  return cells[(v + r) * (r * 2 + 1) + u + r] ?? 3;
}
export function stepField(source: FieldState, direction: number): FieldState {
  if (!Number.isInteger(direction) || direction < 0 || direction > 4)
    throw Error("Invalid original field direction");
  const state: FieldState = {
    ...source,
    anchor: [...source.anchor],
    camera: [...source.camera],
    offset: [...source.offset],
    trail: source.trail.map((t) => [...t]),
    checks: source.checks.map((t) => [...t]),
    actors: source.actors.map((a) => [...a]),
    frame: (source.frame + 1) >>> 0,
  };
  let walking = false;
  if (direction !== 4) {
    const dx = direction === 0 || direction === 1 ? -16 : 16;
    const dy = direction === 1 || direction === 2 ? -8 : 8;
    const x = state.camera[0] + state.offset[0],
      y = state.camera[1] + state.offset[1];
    state.direction = direction;
    if (!(fieldCell(state, x + dx, y + dy) & 1)) {
      for (let i = 3; i >= 0; i--) state.trail[i + 1] = [...state.trail[i]];
      state.trail[0] = [x, y, direction];
      state.camera[0] += dx;
      state.camera[1] += dy;
      walking = true;
    }
  }
  if (walking) {
    state.step = (state.step + 1) % 4;
    const leaderFrame = state.step & 1 ? (state.step + 1) / 2 : 0;
    const followerFrame = state.step & 1 ? 3 - leaderFrame : 0;
    const leader = state.actors[0];
    leader[3] = state.camera[0] + state.offset[0];
    leader[4] = state.camera[1] + state.offset[1];
    leader[5] =
      leader[6] === 4
        ? state.direction * 4 + state.step
        : state.direction * 3 + leaderFrame;
    for (let i = 1; i < state.party; i++) {
      const actor = state.actors[i],
        trail = state.trail[1],
        dir = trail[2];
      actor[3] = trail[0];
      actor[4] = trail[1];
      if (i === 2) {
        actor[3] += dir === 1 || dir === 3 ? -16 : 16;
        actor[4] += 8;
      } else {
        actor[3] += dir === 1 || dir === 0 ? 16 : -16;
        actor[4] += dir === 1 || dir === 2 ? 8 : -8;
      }
      if (fieldCell(state, actor[3], actor[4]) & 1) {
        actor[3] = trail[0];
        actor[4] = trail[1];
      }
      actor[5] =
        actor[6] === 4
          ? state.trail[2][2] * 4 + state.step
          : state.trail[2][2] * 3 + leaderFrame;
    }
    for (let i = 1; i <= state.followers; i++) {
      const actor = state.actors[state.party + i - 1],
        trail = state.trail[2 + i];
      actor[3] = trail[0];
      actor[4] = trail[1];
      actor[5] = trail[2] * 3 + followerFrame;
    }
  } else {
    for (let i = 0; i < state.party; i++) {
      const actor = state.actors[i],
        frames = actor[6] || 3;
      actor[5] = (i === 0 ? state.direction : state.trail[2][2]) * frames;
    }
    for (let i = 1; i <= state.followers; i++)
      state.actors[state.party + i - 1][5] = state.trail[2 + i][2] * 3;
    state.step = (state.step & 2) ^ 2;
  }
  return state;
}
export function drawField(
  state: FieldState,
  geometry: FieldGeometry,
): FieldDraw[] {
  const draws: FieldDraw[] = [],
    [cameraX, cameraY] = state.camera;
  for (const actor of state.actors) {
    const [kind, id, asset, worldX, worldY, frame, , rawLayer] = actor;
    const rect = geometry.sprite(asset, frame);
    if (!rect || !rect.width || !rect.height) continue;
    const x = worldX - cameraX - Math.trunc(rect.width / 2);
    const sortY = worldY - cameraY + rawLayer + (kind === 1 ? 9 : 10);
    const layer = rawLayer + (kind === 1 ? 2 : 6);
    if (kind === 1) {
      const vy = sortY - rect.height - rawLayer + 2;
      if (x >= 320 || x < -rect.width || vy >= 200 || vy < -rect.height)
        continue;
    }
    draws.push({
      kind: 0,
      a: asset,
      b: frame,
      c: id,
      d: kind,
      x,
      baseY: sortY - layer,
      sortY,
    });
    const sx = cameraX + x - Math.trunc(layer / 2),
      sy = cameraY + sortY - layer;
    const sh = sx % 32 ? 1 : 0;
    const left = Math.trunc((sx - Math.trunc(rect.width / 2)) / 32);
    const right = Math.trunc((sx + Math.trunc(rect.width / 2)) / 32);
    for (
      let y = Math.trunc((sy - rect.height - 15) / 16);
      y <= Math.trunc(sy / 16);
      y++
    ) {
      for (let tx = left; tx <= right; tx++) {
        let dx = 0,
          dy = 0,
          half = 0;
        for (let i = tx === left ? 0 : 3; i < 5; i++) {
          if (i === 0) {
            dx = tx;
            dy = y;
            half = sh;
          } else if (i === 1) dx = tx - 1;
          else if (i === 2) {
            dx = sh ? tx : tx - 1;
            dy = sh ? y + 1 : y;
            half = 1 - sh;
          } else if (i === 3) {
            dx = tx + 1;
            dy = y;
            half = sh;
          } else {
            dx = sh ? tx + 1 : tx;
            dy = sh ? y + 1 : y;
            half = 1 - sh;
          }
          for (let tileLayer = 0; tileLayer < 2; tileLayer++) {
            const height = geometry.height(dx, dy, half, tileLayer);
            if (height > 0 && (dy + height) * 16 + half * 8 >= sy)
              draws.push({
                kind: 1,
                a: dx,
                b: dy,
                c: half,
                d: tileLayer,
                x: dx * 32 + half * 16 - 16 - cameraX,
                baseY: dy * 16 + half * 8 + 7 - cameraY,
                sortY:
                  dy * 16 + half * 8 + 7 + tileLayer + height * 8 - cameraY,
              });
          }
        }
      }
    }
  }
  return draws.sort((a, b) => a.sortY - b.sortY);
}
