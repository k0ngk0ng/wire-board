import type { Kind, Point } from "./types";

export const BOARD_WIDTH = 10;
export const HIDDEN_ROWS = 2;
export const VISIBLE_ROWS = 20;
export const BOARD_ROWS = HIDDEN_ROWS + VISIBLE_ROWS;

const baseShapes: Record<Kind, string[]> = {
  1: ["....", "####", "....", "...."],
  2: ["#..", "###", "..."],
  3: ["..#", "###", "..."],
  4: ["##", "##"],
  5: [".##", "##.", "..."],
  6: [".#.", "###", "..."],
  7: ["##.", ".##", "..."],
};

function rotateCW(rows: string[]): string[] {
  const n = rows.length;
  const out = Array.from({ length: n }, () => Array(n).fill("."));
  for (let y = 0; y < n; y++)
    for (let x = 0; x < n; x++) out[x][n - 1 - y] = rows[y][x];
  return out.map((row) => row.join(""));
}

const cache: Record<number, Point[][]> = {};
for (let kind = 1 as Kind; kind <= 7; kind++) {
  let rows = baseShapes[kind];
  const rotations: Point[][] = [];
  for (let rot = 0; rot < 4; rot++) {
    const cells: Point[] = [];
    rows.forEach((row, y) => {
      for (let x = 0; x < row.length; x++)
        if (row[x] === "#") cells.push({ x, y });
    });
    rotations.push(cells);
    rows = rotateCW(rows);
  }
  cache[kind] = rotations;
}

export function shapeCells(kind: Kind, rot: number): Point[] {
  return cache[kind][((rot % 4) + 4) % 4];
}

export function fullBoard(visible: string[]): string[] {
  const hidden = "0".repeat(BOARD_WIDTH);
  return [hidden, hidden, ...visible];
}

export function fits(
  board: string[],
  kind: Kind,
  rot: number,
  x: number,
  y: number,
): boolean {
  for (const pt of shapeCells(kind, rot)) {
    const bx = x + pt.x;
    const by = y + pt.y;
    if (bx < 0 || bx >= BOARD_WIDTH || by < 0 || by >= BOARD_ROWS)
      return false;
    if (board[by]?.[bx] !== "0") return false;
  }
  return true;
}

export function dropY(
  board: string[],
  kind: Kind,
  rot: number,
  x: number,
  y: number,
): number {
  if (!fits(board, kind, rot, x, y)) return y;
  while (fits(board, kind, rot, x, y + 1)) y++;
  return y;
}

export function tryMove(
  board: string[],
  piece: { kind: Kind; rot: number; x: number; y: number },
  dx: number,
): boolean {
  if (!fits(board, piece.kind, piece.rot, piece.x + dx, piece.y)) return false;
  piece.x += dx;
  return true;
}

type Kick = { x: number; y: number };

const kicksJLSTZ: Record<number, Kick[]> = {
  1: [{ x: 0, y: 0 }, { x: -1, y: 0 }, { x: -1, y: -1 }, { x: 0, y: 2 }, { x: -1, y: 2 }],
  4: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: 1, y: 1 }, { x: 0, y: -2 }, { x: 1, y: -2 }],
  6: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: 1, y: 1 }, { x: 0, y: -2 }, { x: 1, y: -2 }],
  9: [{ x: 0, y: 0 }, { x: -1, y: 0 }, { x: -1, y: -1 }, { x: 0, y: 2 }, { x: -1, y: 2 }],
  11: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: 1, y: -1 }, { x: 0, y: 2 }, { x: 1, y: 2 }],
  14: [{ x: 0, y: 0 }, { x: -1, y: 0 }, { x: -1, y: 1 }, { x: 0, y: -2 }, { x: -1, y: -2 }],
  12: [{ x: 0, y: 0 }, { x: -1, y: 0 }, { x: -1, y: 1 }, { x: 0, y: -2 }, { x: -1, y: -2 }],
  3: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: 1, y: -1 }, { x: 0, y: 2 }, { x: 1, y: 2 }],
};

const kicksI: Record<number, Kick[]> = {
  1: [{ x: 0, y: 0 }, { x: -2, y: 0 }, { x: 1, y: 0 }, { x: -2, y: -1 }, { x: 1, y: 2 }],
  4: [{ x: 0, y: 0 }, { x: 2, y: 0 }, { x: -1, y: 0 }, { x: 2, y: 1 }, { x: -1, y: -2 }],
  6: [{ x: 0, y: 0 }, { x: -1, y: 0 }, { x: 2, y: 0 }, { x: -1, y: 2 }, { x: 2, y: -1 }],
  9: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: -2, y: 0 }, { x: 1, y: -2 }, { x: -2, y: 1 }],
  11: [{ x: 0, y: 0 }, { x: 2, y: 0 }, { x: -1, y: 0 }, { x: 2, y: 1 }, { x: -1, y: -2 }],
  14: [{ x: 0, y: 0 }, { x: -2, y: 0 }, { x: 1, y: 0 }, { x: -2, y: -1 }, { x: 1, y: 2 }],
  12: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: -2, y: 0 }, { x: 1, y: -2 }, { x: -2, y: 1 }],
  3: [{ x: 0, y: 0 }, { x: -1, y: 0 }, { x: 2, y: 0 }, { x: -1, y: 2 }, { x: 2, y: -1 }],
};

export function tryRotate(
  board: string[],
  piece: { kind: Kind; rot: number; x: number; y: number },
  dir: number,
): boolean {
  if (piece.kind === 4) return false;
  const target = (piece.rot + dir + 4) % 4;
  const table = piece.kind === 1 ? kicksI : kicksJLSTZ;
  for (const kick of table[piece.rot * 4 + target] ?? []) {
    const x = piece.x + kick.x;
    const y = piece.y + kick.y;
    if (fits(board, piece.kind, target, x, y)) {
      piece.rot = target;
      piece.x = x;
      piece.y = y;
      return true;
    }
  }
  return false;
}
