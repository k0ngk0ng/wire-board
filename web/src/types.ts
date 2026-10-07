export type User = {
  id: string;
  name: string;
  role?: "player" | "admin" | "superadmin";
};
export type Seat = User & {
  ready: boolean;
  left: boolean;
  bot?: boolean;
  autoPlay?: boolean;
};
export type ChatMessage = {
  spectator?: boolean;
  id: string;
  sender: User;
  text: string;
  sentAt: number;
};
export type Card = {
  orient?:
    "gold" | "copy" | "copy_cascade" | "double" | "cascade" | "sacrifice";
  bonusCount?: number;
  sacrificeColor?: number;
  id: number;
  tier: number;
  points: number;
  color: number;
  cost: number[];
};
export type GemCity = {
  tile: number;
  side: number;
  name: string;
  points: number;
  cost: number[];
  any?: number;
};
export type Noble = { id: number; cost: number[] };
export type SplendorCardEvent = {
  orient?: boolean;
  id: number;
  player: number;
  action: "buy" | "reserve" | "noble" | "free";
  source: "market" | "deck" | "reserved" | "nobles";
  tier: number;
  slot: number;
  card?: Card;
  noble?: Noble;
};
export type SplendorTokenEvent = {
  id: number;
  player: number;
  action: "take" | "gold" | "pay" | "return";
  tokens: number[];
};
export type GemPlayer = {
  tradingPosts?: number[];
  eliminated?: boolean;
  tokens: number[];
  bonus: number[];
  reserved?: Card[];
  reservedCount?: number;
  cards: Card[];
  nobles: Noble[];
  score: number;
};
export type RailMapSpec = {
  id: string;
  name: string;
  minPlayers: number;
  maxPlayers: number;
  width: number;
  height: number;
  trains: number;
  doubleMin: number;
  setupTickets: number;
  longTickets: boolean;
  initialReturn: boolean;
  additionalReturn: boolean;
  wildSingle: boolean;
  wildRule: string;
  bonus: string;
  stations: number;
  art: string;
  rules: string[];
};
export type RailPayment = { color: number; tokens: number[] };
export type Ticket = {
  art?: number;
  long?: boolean;
  options?: { to: number; points: number }[];
  value?: number;
  mandala?: boolean;
  id: number;
  a: number;
  b: number;
  points: number;
  complete: boolean;
};
export type RailPlayer = {
  stations?: number[];
  stationRoutes?: number[];
  stationScore?: number;
  mandalaCount?: number;
  mandalaScore?: number;
  network?: number;
  mountainTrains?: number;
  mountainRoutes?: number;
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
  dota?: DotaState;
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
    cities?: GemCity[];
    cityEligibility?: number[][];
    options?: SplendorOptions;
    tradingPostRules?: {
      id: number;
      name: string;
      description: string;
      cost: number[];
    }[];
    postChoices?: number[];
    reserveChoice?: Card[];
    effects?: { kind: string; exclude: number; card?: number; tier?: number }[];
    freeCardChoices?: Card[];
    strongholds?: Record<string, { player: number; count: number }>;
    strongholdActions?: {
      type: string;
      choice: string;
      card: number;
      target: number;
    }[];
    conquestCard?: number;
    startPlayer?: number;
    cardEventId?: number;
    cardEvents?: SplendorCardEvent[];
    tokenEventId?: number;
    tokenEvents?: SplendorTokenEvent[];
    bank: number[];
    market: Card[][];
    nobles: Noble[];
    players: GemPlayer[];
    remaining: number[];
    lastRound: boolean;
  };
  rail?: {
    map?: string;
    mapInfo: RailMapSpec;
    payments?: Record<string, RailPayment[]>;
    tunnel?: {
      route: number;
      color: number;
      base: number[];
      revealed: number[];
      extra: number;
      wildOnly: boolean;
    };
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
export type CatanNewWorldMap = {
  hexes: { resource: number; number: number }[];
};
export type Room = {
  catanFriendlyRobberAvailability?: {
    allowed: boolean;
    reason: string;
    minPlayers: number;
  };
  catanFriendlyRobber?: { enabled: boolean; rules: string };
  catanHarbors?: { enabled: boolean; rules: string };
  catanCitiesKnights?: { layout: "variable"; rules: string };
  catanBaseConfiguration?: { layout: "fixed" | "variable"; rules: string };
  catanBaseLayouts?: string[];
  catanSeafarers?: { scenario: string; layout: string; rules: string };
  catanSeafarersChoices?: {
    id: string;
    name: string;
    layouts: string[];
    victoryPoints: number;
  }[];
  catanNewWorldMap?: CatanNewWorldMap;
  catanOptions?: CatanOptions;
  catanTwoRules?: string;
  catanTwoScenario?: string;
  splendorOptions?: SplendorOptions;
  sanguoshaOptions?: SGOptions;
  railMap?: string;
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
  closeReason?: string;
  locked: boolean;
  version: number;
  you: number;
  game?: Game;
  updated: number;
};

export type SplendorOptions = {
  extraNobles?: boolean;
  cities?: boolean;
  orient?: boolean;
  rules?: string;
  tradingPosts?: boolean;
  strongholds?: boolean;
};
export type State = {
  availableGames: string[];
  railMaps: RailMapSpec[];
  user: User;
  rooms: Room[];
  room?: Room;
  serverNow: number;
  receivedAt: number;
  assetsBaseURL?: string;
};
export type City = {
  kind?: string;
  country?: string;
  id: number;
  name: string;
  x: number;
  y: number;
  label?: number[];
};
export type Route = {
  tunnel?: boolean;
  ferry?: number;
  mountain?: number;
  substitute?: number;
  id: number;
  a: number;
  b: number;
  length: number;
  color: number;
  segments?: { x: number; y: number; angle: number }[];
};
export type Catalog = {
  map: RailMapSpec;
  cities: City[];
  routes: Route[];
  tickets: Ticket[];
};
export type Act = (action: Record<string, unknown>) => Promise<void>;

export type CatanPlayer = {
  helper?: {
    id: number;
    moon: boolean;
    acquiredTurn: number;
    usedTurn: number;
  };
  helperReady?: boolean;
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
  bridgesLeft?: number;
  shipsLeft?: number;
  settlementsLeft: number;
  citiesLeft: number;
};
export type CatanKnight = {
  owner: number;
  vertex: number;
  strength: number;
  active: boolean;
  activatedAt: number;
  promotedAt: number;
};
export type CatanProgressEvent = {
  id: number;
  kind: "draw" | "play" | "return" | "transfer";
  player: number;
  other: number;
  track: number;
  count: number;
  card?: number;
};
export type CatanCitiesKnights = {
  progressEventId?: number;
  progressEvents?: CatanProgressEvent[];
  rules: string;
  layout?: string;
  chase?: "robber" | "pirate";
  pirateStart?: number;
  barbarianPosition: number;
  eventDie: number;
  invasions: number;
  actionSerial: number;
  walls: number[];
  metropolises: number[];
  fallenCities: number[];
  knights: CatanKnight[];
  progressRemaining: number[];
  merchant?: { owner: number; tile: number };
  tradePowers?: { player: number; fleets: number[]; harbors: number[][] };
  players: {
    improvements: number[];
    progress?: number[];
    progressCount: number;
    publicProgress: number[];
    defenderPoints: number;
    progressPoints: number;
  }[];
  pending?: {
    kind: string;
    players: number[];
    track: number;
    target: number;
    color: number;
    ship?: boolean;
    knight?: CatanKnight;
    resources?: number[];
    progress?: number[];
  };
};
export type CatanRevealedEvent = {
  kind: string;
  production: number;
  red: number;
  face: number;
  rollId: number;
  productionStarted: boolean;
};
export type CatanWagon = { edge: number; from: number };
export type CatanTwoChoice = { owner: number; vertex: number; edge: number };
export type CatanAttackKnight = { player: number; edge: number };
export type CatanAttackMove = { from: number; to: number; wheat?: boolean };
export type CatanAttack = {
  rules: string;
  map: { castles: number[]; coast: number[]; barbarians: number; gold: number };
  barbarians: number[];
  knights: CatanAttackKnight[];
  prisoners: number[];
  gold: number[];
  goldBank: number;
  bought: number;
  supply: number;
  conquered: number[];
  conqueredBuildings: number[];
  knightsLeft: number[];
  devRemaining: number;
  discard: string[];
  canAct: boolean;
  canBuyCard: boolean;
  pending?: { id: number; player: number; card: string };
  endPlan?: { id: number; player: number; moves?: CatanAttackMove[] };
  moveChoices?: {
    from: number;
    required: boolean;
    normal: number[];
    wheat: number[];
  }[];
  previewKnights?: CatanAttackKnight[];
  previewWheat?: number;
  canConfirm?: boolean;
  targets?: number[];
  edges?: number[];
  sources?: number[];
  destinations?: number[];
  fromBoard?: number;
  end?: {
    id: number;
    player: number;
    moves: CatanAttackMove[];
    battles: {
      tile: number;
      barbarians: number;
      knights: CatanAttackKnight[];
      prisoners: number[];
      gold: number[];
      contests?: { players: number[]; dice: number[] }[];
      lossDie?: number;
      lost?: CatanAttackKnight[];
    }[];
  };
};
export type CatanTransport = {
  rules: string;
  map: {
    sites: {
      tile: number;
      kind: string;
      center: number;
      paths: number[];
      blocked: number[];
    }[];
  };
  state: {
    wagons: {
      position: number;
      level: number;
      cargo?: { id: number; origin: string; cargo: string };
      delivered: number;
      points: number;
      gold: number;
    }[];
    supply: number[];
    goldBank: number;
    barbarians: number[];
    active: number;
    sequence: number;
    arrivalResolved: boolean;
    travel?: {
      player: number;
      level: number;
      position: number;
      points: number;
      wheatUsed: boolean;
      neutralTolls?: number;
      attempted: boolean[];
      pending: number;
      arrived: number;
      ended: boolean;
    };
  };
  canAct: boolean;
  canDeliver: boolean;
  swift: boolean;
  moves: number;
  barbarianPending: boolean;
  barbarianSequence: number;
  bought: number;
  choices: {
    steps?: {
      edge: number;
      to: number;
      mp: number;
      toll: number;
      pay: number;
      bank?: number;
      neutral?: boolean;
    }[];
    drive?: number[];
    relocate?: number[];
    canWheat?: boolean;
    canStop?: boolean;
    upgradeCost?: number[];
    canUpgrade?: boolean;
    buy?: number[];
    sell?: number[];
    rates?: number[];
  };
};
export type CatanState = {
  medicineHarbors?: number[];
  explorer?: import("./catan-explorer-state").ExplorerView;
  transport?: CatanTransport;
  developmentNames?: string[];
  attack?: CatanAttack;
  two?: {
    rules: string;
    rolls: number[];
    sequence: number;
    tokens: number[];
    bank: number;
    spent: boolean;
    knightExchanged: boolean;
    pending?: { kind: "road" | "settlement" | "bridge"; resume: string };
    trade?: { resume: string; drawn?: number[] };
    actor: number;
    canAct: boolean;
    cost?: number;
    tokenWindow?: boolean;
    choices?: CatanTwoChoice[];
    retreatTiles?: number[];
    retreatEdges?: number[];
    retreatCost?: number;
    neutralRoadLengths: number[];
  };
  caravans?: {
    rules?: string;
    map: { wateringHoles: number[]; starts: CatanWagon[]; supply: number };
    wagons: CatanWagon[];
    built: boolean;
    sequence: number;
    remaining: number;
    actor: number;
    canAct: boolean;
    choices?: CatanWagon[];
    pending?: {
      kind: "bid" | "vote" | "place";
      active: number;
      order: number[];
      cursor: number;
      chooser: number;
      bids: (number[] | null)[];
      votes: (CatanWagon | null)[];
      two?: { start: number; first?: CatanWagon; train?: number };
    };
  };
  rivers?: {
    rules?: string;
    map: {
      channels: { tiles: number[]; outlet: number }[];
      bridges: number[];
      swamps: number[];
      doubleNumberTile: number;
    };
    gold: number[];
    bank: number;
    bought: number;
    richest: number;
    poor: number[] | null;
  };
  fishing?: {
    worldSetup?: {
      index: number;
      total: number;
      remaining: number;
      current?: number;
    };
    map: {
      lakes: { tile: number; numbers: number[] }[];
      extraNumbers?: { tile: number; number: number }[];
      grounds: {
        number: number;
        edges: number[];
        vertices: number[];
        seaTile?: number;
      }[];
    };
    tokens: {
      remaining: number;
      discard: { id: number; fish: number }[];
      players: { count: number; tokens?: { id: number; fish: number }[] }[];
      bootOwner: number;
      responder?: number;
    };
    victoryTargets: number[];
    canReplace: boolean;
    legal: {
      costs: Record<string, number>;
      actions: string[];
      resources: number[];
      targets: number[];
      roads: number[];
      bootTargets: number[];
      progressTracks?: number[];
      ships?: number[];
    };
  };
  revealedEvent?: CatanRevealedEvent;
  cardEvent?: {
    kind: string;
    production: number;
    red: number;
    face: number;
    players: number[];
    targets?: number[];
    canSkip?: boolean;
    ownGift?: { from: number; to: number; color: number };
  };
  victoryTarget?: number;
  friendlyRobber?: { rules: string; protectedPlayers: number[] };
  harbors?: { rules: string; owner: number; points: number[] };
  citiesKnights?: CatanCitiesKnights;
  progressRules?: {
    id: number;
    name: string;
    track: number;
    count: number;
    victory: boolean;
  }[];
  progressPlayable?: number[];
  inventionTiles?: number[];
  smithingOptions?: number[][];
  merchantTiles?: number[];
  guildDuesTargets?: number[];
  intrigueTargets?: number[];
  diplomacyRoads?: number[];
  diplomacyPlacements?: number[];
  treasonPlacements?: number[];
  knightMoves?: Record<string, number[]>;
  seafarers?: {
    variable?: boolean;
    rules?: string;
    layout?: string;
    newWorld?: {
      index: number;
      total: number;
      remaining: number;
      current?: number;
    };
    wonders?: {
      cards: { id: number; owner: number; level: number }[];
      markers: { card: number; vertex: number }[];
      setupBlocked: number[];
    };
    pirateIslands?: {
      colors: number[];
      homeTiles: number[];
      fleetPath: number[];
      safeTile: number;
      fortresses: {
        root: number;
        route: number[];
        startVertex: number;
        startShip: number;
        beachhead: number;
        vertex: number;
        strength: number;
      }[];
      raid?: { rewards: number[]; total: number };
      battle?: {
        id: number;
        player: number;
        die: number;
        warships: number;
        removed: number[];
        remaining: number;
      };
    };
    islands?: number[];
    startIslands?: number[];
    scenario: string;
    victoryPoints: number;
    pirate: number;
    islandBonus?: number;
    movedShip?: boolean;
    fog?: { remaining: number; startTiles: number[] };
    cloth?: {
      stock: number;
      held: number[];
      homeTiles: number[];
      emptyLimit: number;
      villages: {
        vertex: number;
        number: number;
        stock: number;
        traders: number[] | null;
      }[];
    };
    tribe?: {
      tokens: number[] | null;
      development: { edge: number }[];
      ports: { edge: number; resource: number }[] | null;
      points: number[];
      heldPorts: (number[] | null)[];
      pending?: { player: number; resume: string };
    };
    seats?: {
      homeIslands?: number[];
      settledIslands?: number[];
      islandPoints?: number;
    }[];
  };
  shipMoves?: Record<string, number[]>;
  wonderRules?: {
    id: number;
    name: string;
    requirement: string;
    cost: number[];
  }[];
  wonderClaims?: number[];
  wonderBuilds?: number[];
  goldPending?: { claims: { player: number; count: number }[]; resume: string };
  hexSize?: number;
  startPlayer?: number;
  paired?: { primary: number; secondary: number; second: boolean };
  baseSetup?: {
    layout: "fixed" | "variable";
    rules: string;
    colors: number[];
    neutralColor: number;
  };
  options?: CatanOptions;
  helperDisplay?: number[];
  helperRules?: {
    id: number;
    name: string;
    originalName: string;
    title: string;
    description: string;
  }[];
  helperPending?: {
    player: number;
    kind: string;
    resume: string;
    cards?: number[];
    resources?: number[];
    target?: number;
    optional?: boolean;
  };
  helperRoadMoves?: Record<string, number[]>;
  tiles: {
    id: number;
    x: number;
    y: number;
    resource: number;
    number: number;
    vertices: number[];
  }[];
  vertices: {
    harbor?: boolean;
    id: number;
    x: number;
    y: number;
    owner: number;
    level: number;
  }[];
  edges: {
    id: number;
    a: number;
    b: number;
    owner: number;
    ship?: boolean;
    warship?: boolean;
    bridge?: boolean;
    damaged?: boolean;
    tiles?: number[];
  }[];
  ports: { edge: number; resource: number }[];
  players: CatanPlayer[];
  bank: number[];
  devRemaining: number;
  robber: number;
  dice: number[];
  rollId: number;
  setupStep: number;
  setupLimit?: number;
  setupVertex: number;
  discardDue: number[];
  victims: number[];
  freeRoads: number;
  playedDev: boolean;
  longestOwner: number;
  armyOwner: number;
  legal: {
    earthquakeRoads?: number[];
    repairRoads?: number[];
    fleeDeserts?: number[];
    eventResources?: number[];
    eventGifts?: number[];
    eventTargets?: number[];
    knightRecruit?: number[];
    knightActivate?: number[];
    knightPromote?: number[];
    knightChase?: number[];
    knightChasePirate?: number[];
    knightRetreat?: number[];
    pillage?: number[];
    settlements: number[];
    cities: number[];
    roads: number[];
    bridges?: number[];
    ships?: number[];
    pirate?: number[];
    robber?: number[];
    ports?: number[];
    fishGrounds?: number[];
  };
  trade?: {
    id: number;
    from: number;
    give: number[];
    take: number[];
    responses: number[];
    goldGive?: number;
    goldTake?: number;
  };
};

export type CatanOptions = {
  fiveSix?: boolean;
  rules?: string;
  helpers?: boolean;
  allHelpers?: boolean;
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

export type SGOptions = {
  standardVersion?: "classic" | "breakthrough";
  mode?: string;
  deck?: string;
  packs?: string[];
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
  deputy?: string;
  shown?: [boolean, boolean];
  lost?: [boolean, boolean];
  ownSkills?: string[];
  ownKingdom?: string;
  canReveal?: boolean;
  knownGenerals?: Record<string, [string, string]>;
  companion?: boolean;
  companionClaimed?: boolean;
  halfHP?: boolean;
  halfClaimed?: boolean;
  disabledSkills?: string[];
  silenced?: boolean;
  handSealed?: boolean;
  yiji?: number[];
  yijiCount?: number;
  qianxun?: number[];
  qianxunCount?: number;
  jieLuoyi?: boolean;
  stars?: number[];
  starCount?: number;
  galeTargets?: number[];
  fogTargets?: number[];
  armorDisabled?: boolean;
  skills?: string[];
  skillsLost?: boolean;
  kingdom?: string;
  female?: boolean;
  fields?: number[];
  avatar?: string;
  avatarSkill?: string;
  avatarCount?: number;
  avatars?: string[];
  marks?: Record<string, number>;
  handLimit?: number;
  flipped?: boolean;
  buqu?: number[];
  chained?: boolean;
  drank?: number;
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
  hegemony?: boolean;
  first?: number;
  companions?: [string, string][];
  armor?: string;
  guhuoKinds?: string[];
  bluff?: {
    player: number;
    declared: string;
    context: string;
    questioned: number[] | null;
    resolved: boolean;
  };
  huangtianGive?: boolean;
  zhibaPindian?: boolean;
  options?: SGOptions;
  revealed?: number[];
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
    general?: string;
    factionCounter?: boolean;
    suit?: number;
    protectedCards?: number[];
    required?: boolean;
    avatarSkills?: Record<string, string[]>;
    moves?: { card: number; targets: number[] }[];
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
    targets?: number[];
  };
};

export type DotaOrder = {
  card: number;
  lane: number;
  target: number;
  buy: string;
};
export type DotaPlayer = {
  hero: string;
  team: number;
  lane: number;
  hp: number;
  gold: number;
  charge: number;
  spent: number;
  gear: string[];
  wounded: boolean;
};
export type DotaCardSpec = { name: string; art: string; text: string };
export type DotaHeroSpec = {
  id: string;
  name: string;
  hp: number;
  passive: string;
  skills: DotaCardSpec[];
};
export type DotaItemSpec = {
  id: string;
  name: string;
  price: number;
  text: string;
  solo: boolean;
};
export type DotaState = {
  rules: string;
  n: number;
  sequence: number;
  captain: number;
  confirmed: boolean[];
  players: DotaPlayer[];
  first: number;
  core: number[];
  towers: number[][];
  track: number[];
  kills: number[];
  locked: boolean[];
  plans: Record<string, DotaOrder>;
  actors: number[] | null;
  legal: DotaOrder[];
  maximum: number[];
  draft: number[];
  draftStep: number;
  history: {
    round: number;
    orders: DotaOrder[];
    before: DotaPlayer[];
    after: DotaPlayer[];
    messages: string[];
    core: number[];
    towers: number[][];
  }[];
  catalog: {
    heroes: DotaHeroSpec[];
    items: DotaItemSpec[];
    common: DotaCardSpec[];
  };
};
