export type Options = {
  bestOf: 1 | 3;
  teams: number[];
  difficulty: "easy" | "normal" | "hard";
  court: "club" | "arena" | "heritage";
};
export type Vec = { x: number; y: number; z: number };
export type Segment = { at: number; until: number; p: Vec; v: Vec; a: Vec };
export type Flight = {
  segments: Segment[];
  events: { type: string; at: number; p: Vec; side: number }[];
  spin: number;
  startAt: number;
  endAt: number;
  receiveAt: number;
  contactStart: number;
  contactEnd: number;
  reason: string;
  winnerSide: number;
  touchedNet: boolean;
  serve: boolean;
};
export type GameEvent = {
  id: number;
  type: string;
  at: number;
  seat: number;
  p: Vec;
  kind?: string;
  score: [number, number];
};
export type Paddle = {
  x: number;
  seq: number;
  at: number;
  rally: number;
  shot: number;
  result: string;
};
export type View = {
  paddles: Paddle[];
  options: Options;
  epoch: string;
  phase:
    | "countdown"
    | "serve"
    | "serve_toss"
    | "rally"
    | "point_break"
    | "game_break"
    | "finished";
  serverAt: number;
  phaseUntil: number;
  serveDeadline: number;
  score: [number, number];
  gamesWon: [number, number];
  game: number;
  games: { score: [number, number]; winner: number }[];
  order: number[];
  ends: number;
  rally: number;
  shot: number;
  expected: number;
  receiver: number;
  lastHitter: number;
  pointWinner: number;
  flight?: Flight;
  events: GameEvent[];
  controls: {
    retrySeq: number;
    canSwing: boolean;
    armed: boolean;
    seq: number;
    openAt: number;
    closeAt: number;
  };
};
export type Stroke = {
  racketX?: number;
  kind: "drive" | "chop" | "block";
  aim: number;
  power: number;
};
export type SwingAction = Stroke & {
  type: "swing";
  epoch: string;
  rally: number;
  shot: number;
  seq: number;
  at: number;
};
export type CancelAction = {
  type: "cancel";
  epoch: string;
  rally: number;
  shot: number;
  seq: number;
};
