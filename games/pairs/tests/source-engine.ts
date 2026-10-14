export type Point = { x: number; y: number };
export type Cell = { kind: 'tile'; icon: number; ice?: boolean } | { kind: 'stone' | 'crate' } | null;
export type Board = { rows: number; cols: number; cells: Cell[] };
export type Pair = [number, number];
export type Mechanic = 'classic' | 'stone' | 'crate' | 'ice' | 'reward' | 'mixed' | 'speed';
export type Level = { number: number; seed: number; name: string; description: string; mechanic: Mechanic; seconds: number; board: Board; solution: Pair[]; challenge: string };

export function random(seed: number) {
  let s = seed | 0;
  return () => { s |= 0; s = s + 0x6D2B79F5 | 0; let t = Math.imul(s ^ s >>> 15, 1 | s); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
}
export function shuffle<T>(items: T[], rng: () => number): T[] {
  const out = [...items];
  for (let i = out.length - 1; i > 0; i--) { const j = Math.floor(rng() * (i + 1)); [out[i], out[j]] = [out[j], out[i]]; }
  return out;
}
export const point = (board: Board, index: number): Point => ({ x: index % board.cols, y: Math.floor(index / board.cols) });
export const tileCount = (board: Board) => board.cells.filter(c => c?.kind === 'tile').length;
const same = (a: Point, b: Point) => a.x === b.x && a.y === b.y;

// Search the complete family of orthogonal routes with at most two corners.
export function findPath(board: Board, a: number, b: number, geometryOnly = false): Point[] | null {
  if (a === b || a < 0 || b < 0 || a >= board.cells.length || b >= board.cells.length) return null;
  const ca = board.cells[a], cb = board.cells[b];
  if (ca?.kind !== 'tile' || cb?.kind !== 'tile') return null;
  if (!geometryOnly && (ca.ice || cb.ice || ca.icon !== cb.icon)) return null;
  const start = point(board, a), end = point(board, b);
  const free = (p: Point) => {
    if (p.x < -1 || p.y < -1 || p.x > board.cols || p.y > board.rows) return false;
    if (same(p, start) || same(p, end)) return true;
    if (p.x === -1 || p.y === -1 || p.x === board.cols || p.y === board.rows) return true;
    return board.cells[p.y * board.cols + p.x] === null;
  };
  const line = (p: Point, q: Point) => {
    if (p.x !== q.x && p.y !== q.y) return false;
    const dx = Math.sign(q.x - p.x), dy = Math.sign(q.y - p.y);
    let x = p.x, y = p.y;
    for (;;) {
      if (!free({ x, y })) return false;
      if (x === q.x && y === q.y) return true;
      x += dx; y += dy;
    }
  };
  const check = (route: Point[]) => {
    const compact = route.filter((p, i) => i === 0 || !same(p, route[i - 1]));
    return compact.every((p, i) => i === 0 || line(compact[i - 1], p)) ? compact : null;
  };
  if (line(start, end)) return [start, end];
  for (const corner of [{ x: start.x, y: end.y }, { x: end.x, y: start.y }]) { const route = check([start, corner, end]); if (route) return route; }
  for (let x = -1; x <= board.cols; x++) { const route = check([start, { x, y: start.y }, { x, y: end.y }, end]); if (route) return route; }
  for (let y = -1; y <= board.rows; y++) { const route = check([start, { x: start.x, y }, { x: end.x, y }, end]); if (route) return route; }
  return null;
}
export function neighbors(board: Board, index: number) {
  const { x, y } = point(board, index);
  return [[x - 1, y], [x + 1, y], [x, y - 1], [x, y + 1]]
    .filter(([cx, cy]) => cx >= 0 && cy >= 0 && cx < board.cols && cy < board.rows).map(([cx, cy]) => cy * board.cols + cx);
}
export function removePair(board: Board, a: number, b: number): Board | null {
  if (!findPath(board, a, b)) return null;
  const cells = board.cells.map(c => c ? { ...c } : null);
  cells[a] = cells[b] = null;
  for (const index of new Set([...neighbors(board, a), ...neighbors(board, b)])) {
    const c = cells[index];
    if (c?.kind === 'crate') cells[index] = null;
    if (c?.kind === 'tile' && c.ice) c.ice = false;
  }
  return { ...board, cells };
}
export function findMove(board: Board): Pair | null {
  const groups = new Map<number, number[]>();
  board.cells.forEach((c, i) => { if (c?.kind === 'tile' && !c.ice) groups.set(c.icon, [...(groups.get(c.icon) || []), i]); });
  for (const list of groups.values()) for (let a = 0; a < list.length; a++) for (let b = a + 1; b < list.length; b++) if (findPath(board, list[a], list[b])) return [list[a], list[b]];
  return null;
}
export function verifySolution(board: Board, solution: Pair[]) {
  let current = board;
  for (const [a, b] of solution) { const next = removePair(current, a, b); if (!next) return false; current = next; }
  return tileCount(current) === 0;
}
const META: Record<Mechanic, [string, string]> = {
  classic: ['初见绿野', '找到相同的图案，让熟悉的快乐重新连起来。'],
  stone: ['林间小径', '石块挡住了去路，试试从棋盘外侧绕过去。'],
  crate: ['木箱秘境', '在木箱旁完成配对，击碎木箱，打开新通道。'],
  ice: ['薄荷冰川', '在冰封图标旁完成配对，让它重新苏醒。'],
  reward: ['缤纷时刻', '轻松一点！连续消除，收获双倍积分。'],
  mixed: ['森林奇遇', '两种机关，一场小小的策略冒险。'],
  speed: ['追光旅途', '跟上闪电的节奏，在更短的时间内清空棋盘。'],
};
export function mechanicFor(number: number): Mechanic {
  if (number <= 3) return 'classic';
  if (number === 4) return 'stone';
  if (number === 5) return 'reward';
  if (number <= 7) return 'crate';
  if (number <= 9) return 'ice';
  return (['mixed', 'classic', 'stone', 'crate', 'ice', 'reward', 'classic', 'mixed', 'speed', 'mixed'] as Mechanic[])[number % 10];
}

// Construct a geometric removal sequence, then assign matching icons to its endpoints.
// Ice is only introduced at positions unlocked by an earlier pair in that sequence.
export function generateLevel(number: number, seed = 240601): Level {
  const rng = random(seed ^ Math.imul(number, 2654435761));
  const mechanic = mechanicFor(number);
  const rows = mechanic === 'reward' ? 6 : number < 12 ? 8 : 10;
  const cols = number < 20 ? 10 : number < 60 ? 12 : 14;
  const pool = number <= 3 ? [1, 2, 3, 4, 5, 6, 7, 9, 10, 12, 14, 17, 18, 24, 26, 27] : Array.from({ length: 51 }, (_, i) => i + 1);
  const icons = shuffle(pool, rng).slice(0, Math.min(28, 12 + Math.floor(number / 3)));
  let finalBoard: Board | null = null, solution: Pair[] = [];
  for (let attempt = 0; attempt < 6; attempt++) {
    const cells: Cell[] = Array.from({ length: rows * cols }, () => ({ kind: 'tile', icon: 1 }));
    const available = shuffle(cells.map((_, i) => i).filter(i => i % cols > 0 && i % cols < cols - 1 && i >= cols && i < (rows - 1) * cols), rng);
    // Always remove cells in pairs, so the tile count remains even.
    const holes = number <= 2 ? 0 : mechanic === 'reward' ? 8 : 4 + 2 * Math.min(4, Math.floor(number / 8));
    available.slice(0, holes).forEach((i, n) => {
      cells[i] = mechanic === 'stone' ? { kind: 'stone' } : mechanic === 'crate' ? { kind: 'crate' } : mechanic === 'mixed' ? { kind: n % 2 ? 'crate' : 'stone' } : null;
    });
    const board: Board = { rows, cols, cells };
    let work: Board = { ...board, cells: cells.map(c => c ? { ...c } : null) };
    const order: Pair[] = [];
    while (tileCount(work)) {
      const candidates = shuffle(work.cells.map((c, i) => c?.kind === 'tile' ? i : -1).filter(i => i >= 0), rng);
      let pair: Pair | null = null;
      for (let tries = 0; tries < 90 && !pair; tries++) {
        const a = candidates[Math.floor(rng() * candidates.length)], b = candidates[Math.floor(rng() * candidates.length)];
        if (findPath(work, a, b, true)) pair = [a, b];
      }
      if (!pair) outer: for (let a = 0; a < candidates.length; a++) for (let b = a + 1; b < candidates.length; b++) if (findPath(work, candidates[a], candidates[b], true)) { pair = [candidates[a], candidates[b]]; break outer; }
      if (!pair) break;
      order.push(pair);
      work = removePair(work, ...pair)!;
    }
    if (tileCount(work)) continue;
    order.forEach(([a, b], i) => { const icon = icons[i % icons.length]; cells[a] = { kind: 'tile', icon }; cells[b] = { kind: 'tile', icon }; });
    if (mechanic === 'ice') {
      const rank = new Map<number, number>(); order.forEach((pair, i) => pair.forEach(p => rank.set(p, i)));
      let count = 0;
      for (const index of shuffle([...rank.keys()], rng)) {
        if (count >= Math.min(10, 4 + Math.floor(number / 10))) break;
        if (neighbors(board, index).some(n => rank.has(n) && rank.get(n)! < rank.get(index)!)) { (cells[index] as { kind: 'tile'; icon: number; ice: boolean }).ice = true; count++; }
      }
    }
    if (verifySolution(board, order)) { finalBoard = board; solution = order; break; }
  }
  if (!finalBoard) {
    const cells: Cell[] = Array(rows * cols).fill(null);
    for (let i = 0; i < cells.length; i += 2) { const icon = icons[(i / 2) % icons.length]; cells[i] = { kind: 'tile', icon }; cells[i + 1] = { kind: 'tile', icon }; solution.push([i, i + 1]); }
    finalBoard = { rows, cols, cells };
  }
  const [name, description] = META[mechanic];
  return { number, seed, name, description, mechanic, board: finalBoard, solution, seconds: mechanic === 'speed' ? 140 : mechanic === 'reward' ? 120 : Math.min(240, 180 + Math.floor(number / 15) * 10), challenge: mechanic === 'reward' || mechanic === 'speed' ? '达成 6 连击' : number % 3 === 0 ? '不使用道具通关' : '剩余 30 秒通关' };
}

export function reshuffle(board: Board, seed: number): { board: Board; relaxed: boolean } {
  const rng = random(seed), positions = board.cells.map((c, i) => c?.kind === 'tile' ? i : -1).filter(i => i >= 0);
  const icons = positions.map(i => (board.cells[i] as { icon: number }).icon);
  if (!positions.length) return { board, relaxed: false };
  for (let attempt = 0; attempt < 40; attempt++) {
    const values = shuffle(icons, rng), cells = board.cells.map(c => c ? { ...c } : null);
    positions.forEach((p, i) => { (cells[p] as { icon: number }).icon = values[i]; });
    const candidate = { ...board, cells };
    if (findMove(candidate)) return { board: candidate, relaxed: false };
  }
  // Last resort: remove obstruction states and regroup remaining pairs in adjacent slots.
  // Pair counts are preserved, including those formerly covered by ice.
  icons.sort((a, b) => a - b);
  const cells: Cell[] = Array(board.cells.length).fill(null);
  icons.forEach((icon, i) => { cells[i] = { kind: 'tile', icon }; });
  return { board: { ...board, cells }, relaxed: true };
}
