import clientCatalog from "../maps/client.json" with { type: "json" };
import tankData from "../data/tanks.json" with { type: "json" };

export type Options = {
  map: string;
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
  kind?: string;
  terrain?: boolean;
};
export type Crater = { x: number; y: number; r: number; seat: number };
export type Shot = {
  id: number;
  seat: number;
  weapon: string;
  arcs: Arc[];
  blasts: Blast[];
  craters?: Crater[];
  at: number;
};
export type Status = {
  kind: string;
  turns: number;
  damage: number;
  source: number;
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
  tankId: number;
  selected: boolean;
  x: number;
  y: number;
  facing: number;
  hp: number;
  maxHp: number;
  fuel: number;
  special: number;
  alive: boolean;
  damage: number;
  kills: number;
  deaths: number;
  statuses?: Status[];
  current: boolean;
};
export type View = {
  options: Options;
  epoch: string;
  phase: "select" | "playing" | "finished";
  mapId: string;
  mapName: string;
  mapWidth: number;
  mapHeight: number;
  mapOffY: number;
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
  craters?: Crater[];
  lastShot?: Shot;
  events: Event[];
  log: string[];
  you: number;
  canSelect: boolean;
  canMove: boolean;
  canFire: boolean;
  selectedTank?: number;
};

export type Sprite = { url: string; w: number; h: number; ax: number; ay: number };
export type TankInfo = {
  id: number;
  name: string;
  hp: number;
  defense: number;
  speed: number;
  primary: number;
  special: number;
  angle: number;
  range: number;
  skills: number[];
  skillName: string | null;
  sprites: Record<string, Sprite>;
  bullet: Sprite | null;
};
export type MapInfo = {
  id: string;
  name: string;
  nameEn: string;
  stage: number;
  width: number;
  height: number;
  offY: number;
  fg: string;
  fg2: string | null;
  bg: string | null;
  thumb: string;
  music: string | null;
  mask: string;
  maskCols: number;
  maskRows: number;
};

export const worldHeight = clientCatalog.worldHeight;
export const cellSize = clientCatalog.cell;
export const maps: MapInfo[] = (clientCatalog.maps as MapInfo[]).sort(
  (a, b) => a.stage - b.stage,
);
export const mapById = new Map(maps.map((m) => [m.id, m]));
export const tanks: TankInfo[] = Object.values(tankData.tanks as Record<string, TankInfo>).sort(
  (a, b) => a.id - b.id,
);
export const tankById = new Map(tanks.map((t) => [t.id, t]));
export const tankPortraits: Record<string, Sprite> = tankData.portraits as Record<string, Sprite>;
export const explosion = tankData.explosion as {
  url: string;
  cellW: number;
  cellH: number;
  count: number;
};
export const sounds: Record<string, string> = tankData.audio as Record<string, string>;

export const defaultMap = "stage-1";
export const teamColors = ["#b6473c", "#4b79a8"];
export const teamNames = ["红队", "蓝队"];

export const local = (path: string) => path;
export const sourceList = (base: string, path: string) => {
  if (!base) return [local(path)];
  return [`${base.replace(/\/$/, "")}${path}`, local(path)];
};

export function clamp(value: number, low: number, high: number) {
  return Math.max(low, Math.min(high, value));
}

export function tankInfo(tankId: number): TankInfo | undefined {
  return tankById.get(tankId);
}

export function tankName(tankId: number) {
  return tankById.get(tankId)?.name ?? "坦克";
}

export function skillName(tankId: number) {
  return tankById.get(tankId)?.skillName ?? "特殊弹";
}

export function statusText(status: Status) {
  return status.kind === "poison"
    ? `中毒 ${status.turns}`
    : status.kind === "burn"
      ? `燃烧 ${status.turns}`
      : status.kind;
}
