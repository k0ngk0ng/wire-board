export type TowerKind = "arrow" | "cannon" | "frost" | "arcane";
export type Options = {
  mode: "campaign" | "endless";
  waves: 10 | 20;
  difficulty: "easy" | "normal" | "hard";
  prepSeconds: 20 | 30 | 45;
};
export type Point = { x: number; y: number };
export type TargetPriority = "first" | "strong" | "support";
export type MapData = {
  width: number;
  height: number;
  path: Point[];
  pads: (Point & {
    id: number;
    terrain?: "plain" | "highground";
    sector?: number;
  })[];
  base?: Point;
  routes?: {
    id: string;
    label: string;
    sector: number;
    points: Point[];
    active: boolean;
  }[];
  regions?: {
    id: number;
    x: number;
    y: number;
    width: number;
    height: number;
    biome: "forest" | "canyon" | "river" | "snow" | "marsh" | "ruins";
    label: string;
  }[];
  features?: (Point & {
    id: string;
    kind: "bridge" | "river" | "highground" | "mine" | "beacon" | "convoy";
    width: number;
    height: number;
    sector: number;
  })[];
};
export type WorldChoice = {
  kind: "route" | "boon";
  stage: number;
  title: string;
  options: { id: string; label: string; description: string; map?: MapData }[];
  votes: string[];
  deadline: number;
};
export type WaveEvent = {
  id: "calm" | "fog" | "frost" | "surge" | "starlight";
  label: string;
  description: string;
  wave: number;
};
export type Objective = Point & {
  hp: number;
  maxHP: number;
  route: string;
  progress: number;
  id: string;
  kind: "mine" | "beacon" | "convoy";
  label: string;
  description: string;
  sector: number;
  cost: number;
  status: "available" | "active" | "completed" | "failed";
  startWave: number;
  endWave: number;
  reward: number;
  baseAtStart: number;
};
export type World = {
  seed: number;
  stage: 1 | 2 | 3 | 4;
  preparedWave: number;
  map: MapData;
  choice: WorldChoice | null;
  boonChoice: WorldChoice | null;
  boons: { power: number; reach: number; fortune: number };
  event: WaveEvent;
  forecast: WaveEvent[];
  objectives: Objective[];
  branches: string[];
  shortcutWave: number;
};
export type Player = {
  gold: number;
  kills: number;
  damage: number;
  ready: boolean;
  pauseVote: boolean;
  resumeVote: boolean;
};
export type Tower = {
  id: number;
  pad: number;
  owner: number;
  kind: TowerKind;
  level: number;
  branch?: "power" | "speed";
  angle: number;
  cooldown: number;
  priority?: TargetPriority;
  effectiveRange?: number;
  effectiveDamage?: number;
  effectiveInterval?: number;
};
export type Enemy = Point & {
  id: number;
  kind: string;
  progress: number;
  hp: number;
  maxHP: number;
  shield: number;
  speed: number;
  slowUntil: number;
  route?: string;
  phase?: number;
  abilityTick?: number;
};
export type Event = Point & {
  id: number;
  type: string;
  toX?: number;
  toY?: number;
  kind?: string;
  owner?: number;
  amount?: number;
  tick: number;
};
export type View = {
  options: Options;
  phase: "prepare" | "battle" | "paused" | "won" | "lost";
  epoch: number;
  wave: number;
  completedWaves: number;
  baseHP: number;
  baseMaxHP: number;
  players: Player[];
  kills: number;
  tick: number;
  serverAt: number;
  nextAt: number;
  elapsedMS: number;
  map: MapData;
  world?: World;
  towers: Tower[];
  enemies: Enemy[];
  events: Event[];
  waveInfo: {
    title: string;
    description: string;
    rule: string;
    total: number;
    spawned: number;
    boss: boolean;
  };
  catalog: {
    kind: TowerKind;
    name: string;
    cost: number;
    range: number;
    damage: number;
    interval: number;
    description: string;
  }[];
  controls: {
    canAct: boolean;
    canReady: boolean;
    canPause: boolean;
    canResume: boolean;
    canRouteVote?: boolean;
    canBoonVote?: boolean;
    objectives?: { objectiveId: string; cost: number; enabled: boolean }[];
    targets?: { towerId: number; enabled: boolean }[];
    builds: { pad: number; kind: TowerKind; cost: number; enabled: boolean }[];
    upgrades: {
      towerId: number;
      cost: number;
      enabled: boolean;
      needsBranch: boolean;
    }[];
    sells: { towerId: number; refund: number; enabled: boolean }[];
  };
};
