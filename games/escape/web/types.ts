export type Options = {
  scenario: "station" | "archive";
  difficulty: "easy" | "normal" | "hard";
  seed?: string;
};
export type Point = { x: number; y: number };
export type Player = Point & {
  seat: number;
  room: string;
  dx: number;
  dy: number;
  seq: number;
  appliedSeq?: number;
  facing: string;
  hp: number;
  carry?: string;
  ready: boolean;
  focus?: string;
  goal?: Point;
  task?: string;
  progress?: number;
};
export type Device = Point & {
  id: string;
  kind: string;
  label: string;
  description: string;
  done: boolean;
  dials?: number[];
  heldUntil?: number;
};
export type Door = Point & { room: string; direction: string };
export type Room = {
  id: string;
  x: number;
  y: number;
  known: boolean;
  kind?: string;
  title?: string;
  doors?: Door[];
  objects?: Device[];
  flood: number;
};
export type Clue = {
  key: string;
  title: string;
  text: string;
  values?: number[];
  room: string;
};
export type View = {
  protocol: number;
  partial?: boolean;
  epoch: string;
  options: Options;
  phase: string;
  tick: number;
  serverAt: number;
  remaining: number;
  power: number;
  circuits: string[];
  players: Player[];
  rooms?: Room[];
  water?: number[];
  journal?: Clue[];
  supplies: { fuses: number; cells: number; meds: number; documents: number };
  objectives?: {
    id: string;
    label: string;
    done: boolean;
    progress?: string;
  }[];
  routes?: { id: string; label: string; available: boolean; reason: string }[];
  rally?: { room: string; until: number; actor: number } | null;
  incident?: {
    room: string;
    kind: string;
    begins: number;
    ends: number;
  } | null;
  events?: {
    id: number;
    kind: string;
    text: string;
    room: string;
    actor: number;
    at: number;
  }[];
  revision: number;
  paused: boolean;
  votes: boolean[];
  canAct: boolean;
  you: number;
  stats: { rescues: number; mistakes: number; explored: number; route: string };
  reason?: string;
  seed?: string;
};
export const scenarioNames = { station: "封锁站", archive: "沉没档案馆" };
export const roleNames = ["机修员", "潜水侦察员", "急救员", "档案研究员"];
export const playerColors = ["#edbb63", "#79c2bc", "#f09780", "#e8dfc2"];
export const directionNames: Record<string, string> = {
  north: "北",
  south: "南",
  west: "西",
  east: "东",
};
export const roomImages = (scenario: Options["scenario"], kind?: string) =>
  `${scenario}-${scenario === "station" ? (kind === "entry" || kind === "signal" || kind === "records" ? "atrium" : "service") : kind === "entry" || kind === "signal" || kind === "records" ? "hall" : "vault"}`;
export function mergeView(previous: View | undefined, next: View): View {
  if (
    previous?.epoch === next.epoch &&
    (next.tick < previous.tick ||
      (next.tick === previous.tick && next.revision < previous.revision))
  )
    return previous;
  return next.partial && previous?.epoch === next.epoch
    ? {
        ...previous,
        ...next,
        rooms: next.rooms ?? previous.rooms,
        journal: next.journal ?? previous.journal,
        objectives: next.objectives ?? previous.objectives,
        routes: next.routes ?? previous.routes,
        events: next.events ?? previous.events,
      }
    : next;
}
