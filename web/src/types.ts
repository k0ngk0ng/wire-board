export type User = { id: string; name: string };
export type Seat = User & { ready: boolean; left: boolean; bot?: boolean };
export type ChatMessage = {
  spectator?: boolean;
  id: string;
  sender: User;
  text: string;
  sentAt: number;
};
export type Card = {
  id: number;
  tier: number;
  points: number;
  color: number;
  cost: number[];
};
export type Noble = { id: number; cost: number[] };
export type SplendorCardEvent = {
  id: number;
  player: number;
  action: "buy" | "reserve";
  source: "market" | "deck" | "reserved";
  tier: number;
  slot: number;
  card?: Card;
};
export type GemPlayer = {
  eliminated?: boolean;
  tokens: number[];
  bonus: number[];
  reserved?: Card[];
  reservedCount?: number;
  cards: Card[];
  nobles: Noble[];
  score: number;
};
export type Ticket = {
  id: number;
  a: number;
  b: number;
  points: number;
  complete: boolean;
};
export type RailPlayer = {
  eliminated?: boolean;
  hand?: number[];
  handCount: number;
  tickets?: Ticket[];
  ticketCount: number;
  trains: number;
  score: number;
  routeScore: number;
  ticketScore: number;
  longest: number;
  bonus: number;
  completed?: number;
};
export type Game = {
  sanguosha?: SanguoshaState;
  carcassonne?: CarcassonneState;
  catan?: CatanState;
  kind: string;
  turn: number;
  phase: string;
  round: number;
  finished: boolean;
  winners: number[];
  log: string[];
  splendor?: {
    startPlayer?: number;
    cardEventId?: number;
    cardEvents?: SplendorCardEvent[];
    bank: number[];
    market: Card[][];
    nobles: Noble[];
    players: GemPlayer[];
    remaining: number[];
    lastRound: boolean;
  };
  rail?: {
    drawId?: number;
    drawEvents?: { id: number; player: number; slot: number; color?: number }[];
    hiddenDrawId?: number;
    hiddenDrawEvents?: { id: number; player: number }[];
    face: number[];
    faceVersion?: number[];
    players: RailPlayer[];
    owners: Record<string, number>;
    pending?: Ticket[];
    setup: boolean;
    setupReady?: boolean[];
    drawn: number;
    lastRemaining: number;
    ticketsRemaining: number;
    remaining: number;
  };
};
export type Room = {
  result?: {
    rated: boolean;
    players: (User & { ratingDelta?: number; rank?: number; bot?: boolean })[];
  };
  spectating?: boolean;
  spectatorCount?: number;
  chat?: ChatMessage[] | null;
  turnDeadline?: number;
  id: string;
  name: string;
  kind: string;
  host: string;
  capacity: number;
  seats: Seat[];
  status: string;
  locked: boolean;
  version: number;
  you: number;
  game?: Game;
  updated: number;
};
export type State = {
  user: User;
  rooms: Room[];
  room?: Room;
  serverNow: number;
  receivedAt: number;
  assetsBaseURL?: string;
};
export type City = {
  id: number;
  name: string;
  x: number;
  y: number;
  label?: number[];
};
export type Route = {
  id: number;
  a: number;
  b: number;
  length: number;
  color: number;
  segments?: { x: number; y: number; angle: number }[];
};
export type Catalog = { cities: City[]; routes: Route[]; tickets: Ticket[] };
export type Act = (action: Record<string, unknown>) => Promise<void>;

export type CatanPlayer = {
  resources?: number[];
  dev?: number[];
  newDev?: number[];
  resourceCount: number;
  devCount: number;
  knights: number;
  roadLength: number;
  score: number;
  publicScore: number;
  eliminated?: boolean;
  rates: number[];
  roadsLeft: number;
  settlementsLeft: number;
  citiesLeft: number;
};
export type CatanState = {
  tiles: {
    id: number;
    x: number;
    y: number;
    resource: number;
    number: number;
    vertices: number[];
  }[];
  vertices: {
    id: number;
    x: number;
    y: number;
    owner: number;
    level: number;
  }[];
  edges: { id: number; a: number; b: number; owner: number }[];
  ports: { edge: number; resource: number }[];
  players: CatanPlayer[];
  bank: number[];
  devRemaining: number;
  robber: number;
  dice: number[];
  rollId: number;
  setupStep: number;
  setupVertex: number;
  discardDue: number[];
  victims: number[];
  freeRoads: number;
  playedDev: boolean;
  longestOwner: number;
  armyOwner: number;
  legal: { settlements: number[]; cities: number[]; roads: number[] };
  trade?: {
    id: number;
    from: number;
    give: number[];
    take: number[];
    responses: number[];
  };
};

export type CarFeature = {
  kind: "city" | "road" | "field" | "monastery";
  ports: number[] | null;
  cities?: number[];
  shields?: number;
  x: number;
  y: number;
};
export type CarDefinition = {
  name: string;
  arts: number[];
  features: CarFeature[];
};
export type CarTile = {
  x: number;
  y: number;
  kind: number;
  art: number;
  rotation: number;
  owner: number;
  feature: number;
};
export type CarPlacement = { x: number; y: number; rotation: number };
export type CarcassonneState = {
  players: { score: number; meeples: number; eliminated?: boolean }[];
  tiles: CarTile[];
  current: number;
  last: number;
  remaining: number;
  discarded: number[];
  catalog: CarDefinition[];
  legal: CarPlacement[];
  meepleChoices: number[];
};

export type SGCard = { id: number; kind: string; suit: number; rank: number };
export type SGGeneral = {
  id: string;
  name: string;
  kingdom: string;
  hp: number;
  female: boolean;
  skills: string[];
};
export type SGPlayer = {
  general: string;
  role?: string;
  hp: number;
  maxHP: number;
  dead: boolean;
  hand?: number[];
  handCount: number;
  equip: number[];
  judgment: { card: number; kind: string }[];
  choices?: string[];
  used: Record<string, number>;
};
export type SanguoshaState = {
  players: SGPlayer[];
  lord: number;
  selecting: boolean;
  remaining: number;
  discardCount: number;
  table: number[];
  grace: number[];
  generals: SGGeneral[];
  cardTypes: Record<
    string,
    { name: string; text: string; slot?: string; range?: number }
  >;
  cards: SGCard[];
  skills: Record<string, { name: string; text: string }>;
  sequence: number;
  distances?: number[];
  range?: number;
  pending?: {
    id: number;
    player: number;
    canRespond?: boolean;
    kind: string;
    message?: string;
    cards?: number[];
    source?: number;
    target?: number;
    effect?: string;
    amount?: number;
    count?: number;
    step?: number;
    choices?: string[];
    wanted?: string;
    ignoreArmor?: boolean;
    cancelled?: boolean;
  };
};
