export type TowerKind = "arrow" | "cannon" | "frost" | "arcane";
export type Options = {
  mode: "campaign" | "endless";
  waves: 10 | 20;
  difficulty: "easy" | "normal" | "hard";
  prepSeconds: 20 | 30 | 45;
};
export type Point = { x: number; y: number };
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
  map: {
    width: number;
    height: number;
    path: Point[];
    pads: (Point & { id: number })[];
  };
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
