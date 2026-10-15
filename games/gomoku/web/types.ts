export type Options = {
  rules: "freestyle" | "exact-five";
  difficulty: "easy" | "standard";
  turnSeconds: number;
};
export type View = {
  options: Options;
  round: string;
  board: number[];
  black: number;
  turn: number;
  ply: number;
  deadline: number;
  phase: "playing" | "finished";
  winner: number;
  reason: string;
  line: number[];
  last: { id: number; seat: number; x: number; y: number; at: number };
  offer: number;
  canPlace: boolean;
  canOffer: boolean;
  canRespond: boolean;
  canResign: boolean;
};
