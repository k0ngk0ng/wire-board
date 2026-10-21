import { classicMapNames } from "./classic-map-options.ts";
export type Options = {
  map: string;
  roundSeconds: number;
};

export type Input = { at: number; move: string; bomb: boolean };

export type Player = {
  character?: number;
  nativeSpeed?: number;
  selected?: boolean;
  mount?: string;
  avatar?: number;
  seat: number;
  seq: number;
  team: number;
  x: number;
  y: number;
  dir: string;
  alive: boolean;
  px?: number;
  py?: number;
  speedMs?: number;
  trappedUntil: number;
  invulnerableUntil: number;
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

export type Blast = {
  x: number;
  y: number;
  seat: number;
  at: number;
  endAt: number;
};
export type Item = {
  id: number;
  x: number;
  y: number;
  type: string;
  at: number;
};
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
  rulesVersion?: number;
  schema: number;
  options: Options;
  phase: "select" | "countdown" | "playing" | "finished";
  selectUntil?: number;
  nativeObjects?: number[];
  boxMotions?: {fromX:number;fromY:number;x:number;y:number;at:number;endAt:number;objectId:number}[];
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
  tiles: number[];
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
  ...classicMapNames,
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
  turtle: "乌龟",
  "turtle-fast": "快乌龟",
  owl: "猫头鹰",
  airship: "飞碟",
  "airship-broken": "故障飞碟",
  octopus: "章鱼",
  "octopus-fast": "快章鱼",
  railcar: "矿车",
};

export const teamNames = ["红队", "蓝队"] as const;

const classicSounds: Record<string, string> = {
  "place.wav": "BombIgnite.wav",
  "explode.wav": "BombExplode.wav",
  "trap.wav": "LockedIn.wav",
  "rescue.wav": "Release.wav",
  "eliminate.wav": "Die.wav",
  "item.wav": "ItemEaten.wav",
  "shield.wav": "Release.wav",
  "sudden.wav": "TimerExpired_0.wav",
  "win.wav": "Win.wav",
  "lose.wav": "Lose.wav",
  "draw.wav": "Draw.wav",
  "start.wav": "GameStart_0.wav",
  "countdown.wav": "TimerGo.wav",
  "battle.wav": "Village.mp3",
};
export const assetURL = (base: string, file: string) =>
  `${base.replace(/\/$/, "")}/bubbleparty/v2/audio/${classicSounds[file] ?? file}`;

export const sources = (base: string, file: string) =>
  base ? [assetURL(base, file), assetURL("", file)] : [assetURL("", file)];
