export type Options = {
  size: number;
  komi: number;
  difficulty: "easy" | "standard";
  turnSeconds: number;
};
export type Event = {
  id: number;
  type: string;
  seat: number;
  pos: number;
  removed: number[] | null;
  at: number;
};
export type Score = {
  black: number;
  white: number;
  neutral: number;
  stones: [number, number];
  territory: [number, number];
  ownership: number[];
};
export type View = {
  options: Options;
  round: string;
  board: number[];
  black: number;
  turn: number;
  ply: number;
  revision: number;
  sequence: number;
  phase: "playing" | "scoring" | "finished";
  winner: number;
  reason: string;
  passes: number;
  captured: [number, number];
  dead: number[];
  confirmed: [boolean, boolean];
  deadline: number;
  deadlines: [number, number];
  score: Score;
  last: Event;
  legal: number[];
  canPlay: boolean;
  canScore: boolean;
  canResign: boolean;
  motion?: { event: Event; duration: number };
};
