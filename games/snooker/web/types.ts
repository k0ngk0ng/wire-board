export type Vec = { x: number; y: number };
export type Ball = { id: number; pos: Vec; down: boolean };
export type Impact = {
  time: number;
  kind: string;
  ball: number;
  other?: number;
  speed?: number;
  pos?: Vec;
};
export type Motion = {
  balls: Ball[];
  first: number[];
  potted: number[];
  off: number[];
  duration: number;
  frames: { time: number; balls: Ball[] }[];
  events: Impact[];
};
export type Flight = {
  id: number;
  start: number;
  end: number;
  seat: number;
  resolved: boolean;
  strike: { angle: number; power: number; top: number; side: number };
  motion: Motion;
};
export type Options = {
  bestOf: number;
  teams: number[];
  aimAssist: "line" | "none";
};
export type Ruling = {
  points: number;
  foul: number;
  reasons: string[];
  miss: boolean;
  fullBall: boolean;
  respot: number[];
};
export type View = {
  concede: boolean[];
  offerTeam: number;
  winner: number;
  finishReason?: string;
  endExplanation?: string;
  ruleset: string;
  options: Options;
  phase: string;
  window: number;
  balls: Ball[];
  turn: number;
  nextSeat: number;
  teams: number[];
  order: number[];
  nextFirst: number[];
  scores: number[];
  framesWon: number[];
  frame: number;
  break: number;
  bestBreak: number[];
  target: string;
  clear: number;
  inHand: boolean;
  freeBall: boolean;
  flight?: Flight;
  foul?: { offender: number; next: number; ruling: Ruling };
  missRun?: { count: number; warned: boolean };
  blackDecider: boolean;
  ready: boolean[];
  resign: boolean[];
  rerack: boolean[];
  results: {
    frame: number;
    concede: boolean[];
    offerTeam: number;
    winner: number;
    scores: number[];
    reason: string;
  }[];
  remaining: number;
  controls: {
    canClaim: boolean;
    canConcede: boolean;
    canAct: boolean;
    canShoot: boolean;
    canPlace: boolean;
  };
};
export const colourNames = [
  "白球",
  "红球",
  "黄球",
  "绿球",
  "棕球",
  "蓝球",
  "粉球",
  "黑球",
];
export const valueOf = (id: number) => (id === 0 ? 0 : id <= 15 ? 1 : id - 14);
export const ballName = (id: number) => colourNames[valueOf(id)];
export const ballInk = [
  "#fffdf3",
  "#b9333d",
  "#eac23e",
  "#298653",
  "#8e533b",
  "#356eb2",
  "#db8eab",
  "#222b30",
];
