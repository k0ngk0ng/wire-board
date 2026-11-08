export type Options = {
  turnSeconds: number;
  preparationSeconds: number;
  maxRounds: number;
  teams: number[];
};
export type Rider = { avatar: number; graphic: number };
export type PetDefinition = {
  stats?: { hp: number; attack: number; defense: number; speed: number };
  loyalty: number;
  id: number;
  name: string;
  graphic: number;
  weights: number[];
  elements: number[];
  riders?: Rider[];
};
export type Skill = {
  id: number;
  name: string;
  family: string;
  description: string;
  power: number;
  hits: number;
  status?: string;
  chance?: number;
};
export type Magic = {
  id: number;
  name: string;
  kind: string;
  mp: number;
  power: number;
  group?: boolean;
  status?: string;
};
export type Equipment = {
  tier?: number;
  maxTier?: number;
  minHits?: number;
  maxHits?: number;
  critical?: number;
  evade?: number;
  resistances?: Record<string, number>;
  id: number;
  name: string;
  slot: string;
  weaponType: number;
  icon: string;
  attack: number;
  defense: number;
  speed: number;
  hp: number;
  mp: number;
  resist?: string;
  magic?: Magic;
};
export type Item = {
  id: number;
  name: string;
  kind: string;
  description: string;
  icon: string;
  power: number;
  group?: boolean;
};
export type Pool = {
  pets: PetDefinition[];
  rides: PetDefinition[];
  skills: Skill[];
  hats: Equipment[];
  weapons: Equipment[];
  clothes: Equipment[];
  accessories: Equipment[];
  hpItems: Item[];
  mpItems: Item[];
  cureItems: Item[];
  tactics: Item[];
};
export type Loadout = {
  elements?: number[];
  pet: number;
  ride: number;
  avatar: number;
  points: number[];
  skills: number[];
  hat: number;
  weapon: number;
  clothes: number;
  accessories: number[];
  hpItem: number;
  mpItem: number;
  cureItem: number;
  tactics: number[];
};
export type Status = {
  poison: number;
  stone: number;
  sleep: number;
  confusion: number;
  drunk: number;
  shield: number;
  haste: number;
  rage: number;
  immune: number;
};
export type Unit = {
  charge?: { skill: number; remaining: number };
  loyalty: number;
  fled?: boolean;
  id: number;
  name: string;
  graphic: number;
  hp: number;
  maxHp: number;
  mp: number;
  maxMp: number;
  attack: number;
  defense: number;
  speed: number;
  elements: number[];
  status: Status;
  guard: boolean;
};
export type InventoryItem = { item: Item; count: number };
export type Player = {
  seat: number;
  team: number;
  deadline: number;
  locked: boolean;
  submitted: boolean;
  battleReady: boolean;
  surrendered: boolean;
  loadout?: Loadout;
  body: Unit;
  pet: Unit;
  ride: Unit;
  riding: boolean;
  damage: number;
  healing: number;
  kills: number;
};
export type Command = { kind: string; id: number; target: number };
export type Plan = { player: Command; pet: Command };
export type BattleEvent = {
  sequence?: number;
  projectiles?: import("./motion").Projectile[];
  motions?: import("./motion").AttackMotion[];
  impactAt?: number;
  reactionDuration?: number;
  originalTarget?: number;
  redirectReason?: string;
  actors?: number[];
  effect?: string;
  id: number;
  kind: string;
  actor: number;
  target: number;
  name: string;
  amount: number;
  rideAmount: number;
  critical?: boolean;
  at: number;
  duration: number;
  players: Player[];
};
export type View = {
  pointBudget: number;
  rules: string;
  session: string;
  window: number;
  phase: string;
  round: number;
  options: Options;
  pool: Pool;
  players: Player[];
  viewer: number;
  self: {
    loadout: Loadout | null;
    inventory: InventoryItem[] | null;
    magic: Magic[] | null;
    plan: Plan | null;
  } | null;
  events: BattleEvent[];
  playback?: { startedAt: number; duration: number; before: Player[] };
  reason: string;
};

export const statusNames: Record<string, string> = {
  poison: "中毒",
  stone: "石化",
  sleep: "睡眠",
  confusion: "混乱",
  drunk: "酒醉",
  shield: "守护",
  haste: "疾风",
  rage: "勇气",
  immune: "抗性",
};
export const weaponNames: Record<number, string> = {
  0: "爪",
  1: "斧",
  2: "棍",
  3: "枪",
  4: "弓",
  17: "回旋镖",
  18: "投掷斧",
  19: "石",
};
