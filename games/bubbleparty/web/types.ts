export type Options = {
  map: string;
  roundSeconds: number;
};

export type Input = { at: number; move: string; bomb: boolean };

export type Player = {
  seat: number;
  seq: number;
  team: number;
  x: number;
  y: number;
  dir: string;
  alive: boolean;
  shield: boolean;
  maxBombs: number;
  range: number;
  eliminations: number;
  crates: number;
  applied: Input;
};

export type Bomb = {
  id: number;
  seat: number;
  team: number;
  x: number;
  y: number;
  range: number;
  at: number;
  explodeAt: number;
};

export type Blast = { x: number; y: number; seat: number; at: number; endAt: number };
export type Item = { id: number; x: number; y: number; type: string; at: number };
export type GameEvent = {
  id: number;
  type: string;
  at: number;
  seat: number;
  x: number;
  y: number;
  kind?: string;
  value?: number;
};

export type View = {
  schema: number;
  options: Options;
  phase: "countdown" | "playing" | "finished";
  epoch: string;
  serverAt: number;
  startAt: number;
  matchEndsAt: number;
  suddenAt: number;
  sudden: boolean;
  safeInset: number;
  mapId: string;
  players: Player[];
  teamSize: number;
  cells: number[];
  bombs: Bomb[];
  blasts: Blast[];
  items: Item[];
  events: GameEvent[];
  log: string[];
  you: number;
  canAct: boolean;
  spectator: boolean;
};

export const mapNames: Record<string, string> = {
  community10: "小区10",
  waterpark: "水乐园",
  forest: "森林公园",
  desert: "沙丘营地",
  random: "随机地图",
};

export const itemNames: Record<string, string> = {
  speed: "速度",
  bomb: "炸弹容量",
  range: "射程",
  shield: "护盾",
};

export const teamNames = ["红队", "蓝队"] as const;

export const assetURL = (base: string, file: string) =>
  `${base.replace(/\/$/, "")}/bubbleparty/v1/audio/${file}`;

export const sources = (base: string, file: string) =>
  base
    ? [assetURL(base, file), assetURL("", file)]
    : [assetURL("", file)];
