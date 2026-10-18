export type Options = {
  mode: "survival" | "sprint";
};

export type Kind = 1 | 2 | 3 | 4 | 5 | 6 | 7;

export type Point = { x: number; y: number };

export type PieceView = {
  id: number;
  kind: Kind;
  rot: number;
  x: number;
  y: number;
  cells: Point[];
  ghostY?: number;
};

export type PlayerView = {
  seat: number;
  alive: boolean;
  eliminatedAt?: number;
  board: string[];
  active?: PieceView;
  lines: number;
  score: number;
  level: number;
  pendingCount: number;
  targetSeat: number;
  queue?: Kind[];
  hold?: Kind;
  canHold?: boolean;
  seq?: number;
};

export type TetrisEvent = {
  id: number;
  type: "start" | "lock" | "clear" | "garbage" | "level" | "eliminate" | "finish";
  at: number;
  seat: number;
  rows?: number[];
  count?: number;
  target?: number;
  level?: number;
  kind?: Kind;
  pieceId?: number;
  applied?: boolean;
};

export type View = {
  options: Options;
  phase: "countdown" | "playing" | "finished";
  epoch: string;
  serverAt: number;
  startAt: number;
  matchEndsAt: number;
  players: PlayerView[];
  events: TetrisEvent[];
  controls: { canPlay: boolean; canHold: boolean };
};

export const modeNames: Record<Options["mode"], string> = {
  survival: "生存赛",
  sprint: "40 行冲刺",
};

export const SPRINT_TARGET = 40;

export const seatColors = [
  "#d06a5e",
  "#5a9bc4",
  "#d4b05a",
  "#6aa88a",
] as const;

export const pieceColors: Record<Kind, string> = {
  1: "#78a9b8",
  2: "#8b8fc4",
  3: "#c59a6b",
  4: "#c9bd72",
  5: "#83b08a",
  6: "#b07fae",
  7: "#c47d79",
};

export const assetURL = (base: string, file: string) =>
  `${base.replace(/\/$/, "")}/tetris/v1/audio/${file}`;

export const sources = (base: string, file: string) =>
  base ? [assetURL(base, file), assetURL("", file)] : [assetURL("", file)];
