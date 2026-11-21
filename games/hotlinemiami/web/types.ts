export type GameOptions = {
  map: string;
  roundSeconds: number;
  characters: string[];
  weapons: string[];
};

export type GameInput = {
  moveX: number;
  moveY: number;
  aim: number;
  fire: boolean;
  throw: boolean;
  pickup: boolean;
  finish: boolean;
  interact: boolean;
};

export type InputAction = GameInput & {
  type: string;
  epoch: string;
  seq: number;
  life: number;
  viewFrame: number;
};

export type Player = {
  seat: number;
  character: string;
  x: number;
  y: number;
  aim: number;
  alive: boolean;
  life: number;
  weapon: string;
  ammo: number;
  score: number;
  kills: number;
  deaths: number;
  respawnFrame: number;
  protectedUntil: number;
  downUntil: number;
  nextAttackFrame: number;
  busyUntil?: number;
  receivedSeq: number;
  appliedSeq: number;
};

export type GameEvent = {
  id: number;
  type: string;
  frame: number;
  seat: number;
  target?: number;
  x?: number;
  y?: number;
  weapon?: string;
};

export type Door = { id: number; open: boolean; x?: number; y?: number; w?: number; h?: number };
export type Bullet = { id: number; x: number; y: number; aim?: number; dx?: number; dy?: number; owner?: number; seat?: number; weapon?: string; thrown?: boolean };
export type Drop = { id: number; x: number; y: number; weapon: string; ammo: number };
export type MapRules = {
  id: string;
  name: string;
  width: number;
  height: number;
  walls: { x: number; y: number; w: number; h: number; kind: string }[];
  spawns: { x: number; y: number }[];
  weaponSpawns: { x: number; y: number; weapon: string }[];
  doors: { id: number; x: number; y: number; w: number; h: number }[];
};

export type View = {
  frame: number;
  endFrame: number;
  mapId: string;
  players: Player[];
  bullets: Bullet[];
  drops: Drop[];
  doors: Door[];
  events: GameEvent[];
  eventId: number;
  finished: boolean;
  options: GameOptions;
  epoch: string;
  serverAt: number;
  startAt: number;
  you: number;
  canAct: boolean;
  brokenGlass?: number[];
  corpses?: { x: number; y: number; aim: number; character: string; frame: number }[];
  map?: MapRules;
  recover?: boolean;
};

export type ImageResource = { file: string; width: number; height: number; bytes: number; sha256: string };
export type SpriteFrame = { x: number; y: number; w: number; h: number; originX?: number; originY?: number };
export type Sprite = { image: string; frames: SpriteFrame[] };
export type CharacterAsset = { id: string; name: string; portrait: string; sprite: string; attack?: string; dead?: string; overlay?: string; overlayFrame?: number; poses?: Record<string, string> };
export type WeaponAsset = { id: string; name: string; sprite: string };
export type MapAsset = { id: string; name: string; image: string; preview: string; width: number; height: number; rules?: MapRules; rulesFile?: string };
export type BinaryResource = { file: string; bytes: number; sha256?: string };
export type AssetManifest = {
  version: number;
  images: Record<string, ImageResource>;
  sprites: Record<string, Sprite>;
  characters: CharacterAsset[];
  weapons: WeaponAsset[];
  maps: MapAsset[];
  sounds?: Record<string, BinaryResource>;
  audio?: Record<string, BinaryResource>;
  music?: BinaryResource;
  runtime?: { wasm: BinaryResource; script: BinaryResource };
};

export type MotionFunction = (x: number, y: number, aim: number, moveX: number, moveY: number, frames: number, doors?: string, brokenGlass?: string) => number[];
export type Viewport = { width: number; height: number };

export { characterNames, weaponNames, mapNames, isMeleeWeapon } from "./content.ts";

export const playerColors = ["#5dd8cf", "#f487b6", "#e9d37a", "#9da4ed"];
