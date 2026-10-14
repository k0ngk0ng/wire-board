export type Cell = {
  kind: "tile" | "stone" | "crate";
  icon?: number;
  ice?: boolean;
} | null;
export type Grid = { rows: number; cols: number; cells: Cell[] };
export type Point = { x: number; y: number };
export type PairEvent = {
  id: number;
  type: string;
  path?: Point[];
  pair?: [number, number];
  at: number;
  recovery?: boolean;
};
export type Player = {
  board: Grid | null;
  tools: { hint: number; shuffle: number; hammer: number };
  revision: number;
  due: number;
  remaining: number;
  combo: number;
  maxCombo: number;
  event: PairEvent;
};
export type View = {
  round: string;
  phase: string;
  startAt: number;
  finishAt: number;
  name: string;
  description: string;
  mechanic: string;
  difficulty: string;
  total: number;
  players: Player[];
  winner: number;
  canAct: boolean;
};
