import type { InputEvent } from "./input";
import type { PlayerView, PairView } from "./types";

// Display only: mirror this game's pairFits/tryRotate, including its kick order.
// Locking, clearing, attacks, scoring and the next pair remain server-owned.
export function predictPair(
  player: PlayerView,
  events: readonly InputEvent[],
): PairView | undefined {
  const original = player.active;
  if (!original || player.resolving || original.cells.length !== 2)
    return original;
  const occupied = new Set<string>();
  for (const gem of player.gems)
    for (let y = gem.y; y < gem.y + gem.h; y++)
      for (let x = gem.x; x < gem.x + gem.w; x++) occupied.add(`${x},${y}`);
  const offsets = [
    [0, -1],
    [1, 0],
    [0, 1],
    [-1, 0],
  ];
  const cells = (x: number, y: number, rot: number) => {
    const [dx, dy] = offsets[rot];
    return [
      { ...original.cells[0], x, y },
      { ...original.cells[1], x: x + dx, y: y + dy },
    ];
  };
  const fits = (x: number, y: number, rot: number) =>
    cells(x, y, rot).every(
      (c) =>
        c.x >= 0 &&
        c.x < 6 &&
        c.y >= 0 &&
        c.y < 12 &&
        !occupied.has(`${c.x},${c.y}`),
    );
  let { x, y, rot } = original;
  for (const event of events) {
    if (event.kind === "left" || event.kind === "right") {
      const next = x + (event.kind === "left" ? -1 : 1);
      if (fits(next, y, rot)) x = next;
    } else if (event.kind === "rotateCW" || event.kind === "rotateCCW") {
      const next = (rot + (event.kind === "rotateCW" ? 1 : 3)) % 4;
      for (const [dx, dy] of [
        [0, 0],
        [-1, 0],
        [1, 0],
        [0, 1],
        [0, -1],
      ])
        if (fits(x + dx, y + dy, next)) {
          x += dx;
          y += dy;
          rot = next;
          break;
        }
    } else if (event.kind === "softDrop") {
      if (fits(x, y + 1, rot)) y++;
    } else if (event.kind === "hardDrop") {
      while (fits(x, y + 1, rot)) y++;
      break;
    }
  }
  let ghost = y;
  while (fits(x, ghost + 1, rot)) ghost++;
  return {
    ...original,
    x,
    y,
    rot,
    cells: cells(x, y, rot),
    ghost: cells(x, ghost, rot).map(({ x, y }) => ({ x, y })),
  };
}
