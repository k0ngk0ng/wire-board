export type Options = {
  map: "hills" | "canyon" | "islands" | "snow" | "volcano" | "random";
  turnSeconds: number;
};

export type Point = { x: number; y: number };
export type Arc = { path: Point[]; impact: Point; hit: number };
export type Blast = {
  x: number;
  y: number;
  r: number;
  seat: number;
  victim: number;
  damage: number;
  heal?: number;
  terrain?: boolean;
};
export type Shot = {
  id: number;
  seat: number;
  weapon: string;
  arcs: Arc[];
  blasts: Blast[];
  at: number;
};
export type Event = {
  id: number;
  type: string;
  seat: number;
  target: number;
  x?: number;
  y?: number;
  amount?: number;
  at: number;
  text?: string;
};
export type Tank = {
  seat: number;
  team: number;
  class: string;
  selected: boolean;
  x: number;
  y: number;
  facing: number;
  hp: number;
  maxHp: number;
  fuel: number;
  special: number;
  shield: number;
  damage: number;
  kills: number;
  deaths: number;
  alive: boolean;
};
export type View = {
  options: Options;
  epoch: string;
  phase: "select" | "playing" | "finished";
  mapId: Exclude<Options["map"], "random">;
  selectUntil: number;
  startAt: number;
  suddenAt: number;
  matchEndsAt: number;
  deadline: number;
  sudden: boolean;
  turn: number;
  current: number;
  wind: number;
  order: number[];
  teamSize: number;
  tanks: Tank[];
  heights: number[];
  lastShot?: Shot;
  events: Event[];
  log: string[];
  you: number;
  canSelect: boolean;
  canMove: boolean;
  canFire: boolean;
  selected?: string;
};

export const mapNames: Record<Options["map"], string> = {
  hills: "层叠丘陵",
  canyon: "回声峡谷",
  islands: "浮岛浅滩",
  snow: "雪原斜面",
  volcano: "火山口",
  random: "随机地图",
};

export const classInfo: Record<
  string,
  { name: string; maxHp: number; move: number; special: string; color: string }
> = {
  balanced: {
    name: "均衡",
    maxHp: 100,
    move: 42,
    special: "双联炮",
    color: "#4d7f72",
  },
  heavy: {
    name: "重装",
    maxHp: 145,
    move: 27,
    special: "重锤",
    color: "#8b6048",
  },
  scout: {
    name: "侦察",
    maxHp: 82,
    move: 66,
    special: "三连发",
    color: "#c49b4a",
  },
  sniper: {
    name: "狙击",
    maxHp: 88,
    move: 31,
    special: "穿甲弹",
    color: "#6d7598",
  },
  support: {
    name: "支援",
    maxHp: 112,
    move: 37,
    special: "维修弹",
    color: "#81755b",
  },
};

export const classOrder = [
  "balanced",
  "heavy",
  "scout",
  "sniper",
  "support",
];

export const teamColors = ["#b65b4d", "#527fa8"];
export const teamNames = ["红队", "蓝队"];

export const local = (file: string) => `/fortressblue/v1/audio/${file}`;
export const sources = (base: string, file: string) =>
  base
    ? [`${base.replace(/\/$/, "")}/fortressblue/v1/audio/${file}`, local(file)]
    : [local(file)];

export function clamp(value: number, low: number, high: number) {
  return Math.max(low, Math.min(high, value));
}
