import catalog from "../maps/catalog.json";

export type Options = {
  map: string;
};

export type MapDef = {
  id: string;
  name: string;
  cells: string[];
};

export const MAPS: MapDef[] = (catalog as { maps: MapDef[] }).maps;
export const RANDOM_MAP = "random";

export type Phase = "select" | "countdown" | "playing" | "finished";
export type Dir = 0 | 1 | 2 | 3;
export type TankClass = "balanced" | "basic" | "fast" | "power" | "armor";

export type TankView = {
  seat: number;
  team: number;
  class?: TankClass;
  selected: boolean;
  x: number;
  y: number;
  dir: Dir;
  hp: number;
  maxHp: number;
  tech: number;
  alive: boolean;
  respawnAt: number;
  shieldUntil: number;
  freezeUntil: number;
  kills: number;
  deaths: number;
  seq: number;
};

export type BulletView = {
  id: number;
  seat: number;
  team: number;
  x: number;
  y: number;
  dir: Dir;
};

export type PowerUpView = {
  id: number;
  type: string;
  x: number;
  y: number;
  expireAt: number;
};

export type BaseView = {
  team: number;
  alive: boolean;
  destroyedAt: number;
  destroyedBy: number;
};

export type GameEvent = {
  id: number;
  type: string;
  at: number;
  seat: number;
  team: number;
  x: number;
  y: number;
  target?: number;
  kind?: string;
  count?: number;
};

export type View = {
  options: Options;
  phase: Phase;
  epoch: string;
  serverAt: number;
  selectUntil: number;
  startAt: number;
  matchEndsAt: number;
  teamSize: number;
  mapId: string;
  broken: string;
  shovelUntil: [number, number];
  tanks: TankView[];
  bullets: BulletView[] | null;
  powerUps: PowerUpView[] | null;
  bases: [BaseView, BaseView];
  events: GameEvent[];
  controls: { canSelect: boolean; canPlay: boolean };
};

export const TANK_ORDER: TankClass[] = ["balanced", "basic", "fast", "power", "armor"];

export const TANK_NAMES: Record<TankClass, string> = {
  balanced: "均衡坦克",
  basic: "基础坦克",
  fast: "快速坦克",
  power: "火力坦克",
  armor: "装甲坦克",
};

export const TANK_STATS: Record<
  TankClass,
  { speed: number; hp: number; bullet: number; blurb: string }
> = {
  balanced: { speed: 44, hp: 1, bullet: 220, blurb: "原版玩家坦克，速度与火力均衡。" },
  basic: { speed: 34, hp: 1, bullet: 200, blurb: "慢速基础型，适合稳守。" },
  fast: { speed: 62, hp: 1, bullet: 200, blurb: "高速突进，抢点和绕后。" },
  power: { speed: 38, hp: 1, bullet: 300, blurb: "高弹速，远距离压制。" },
  armor: { speed: 28, hp: 4, bullet: 200, blurb: "四层装甲，慢速推进。" },
};

export const TEAM_NAMES = ["上方队伍", "下方队伍"];
export const TEAM_COLORS = ["#e0784f", "#5aa9d6"];

export const POWERUP_NAMES: Record<string, string> = {
  star: "星",
  grenade: "手雷",
  helmet: "头盔",
  shovel: "铲子",
  tank: "备用坦克",
  clock: "时钟",
};

export const eventSound = (type: string): string | null => {
  switch (type) {
    case "fire":
      return "sfx-fire";
    case "brick":
      return "sfx-brick";
    case "steel":
      return "sfx-steel";
    case "kill":
      return "sfx-explode";
    case "hit":
      return "sfx-explode";
    case "powerup_spawn":
    case "powerup_pickup":
      return "sfx-powerup";
    case "shield":
      return "sfx-shield";
    case "base_destroyed":
      return "sfx-base";
    default:
      return null;
  }
};

export const mapById = (id: string): MapDef | undefined =>
  MAPS.find((m) => m.id === id);

export const SUB_COLS = 26;
export const SUB_ROWS = 26;

export const decodeBroken = (value: string): Uint8Array => {
  const out = new Uint8Array(SUB_COLS * SUB_ROWS);
  if (!value) return out;
  try {
    const raw = atob(value);
    for (let i = 0; i < out.length && i < raw.length; i++)
      out[i] = raw.charCodeAt(i) ? 1 : 0;
  } catch {
    // A malformed map delta is rendered as intact terrain; gameplay remains
    // server-authoritative and the next snapshot repairs the view.
  }
  return out;
};

export const cellAt = (map: MapDef, x: number, y: number): number => {
  const row = map.cells[y];
  return row ? Number(row[x] ?? 0) : 0;
};

export const phaseLabel = (phase: Phase) => {
  switch (phase) {
    case "select":
      return "选择坦克";
    case "countdown":
      return "准备开战";
    case "finished":
      return "对局结束";
    default:
      return "对局进行中";
  }
};
