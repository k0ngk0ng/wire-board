import type { CatanState, Room } from "./types";
import { catanCardNames } from "./catan-cards.ts";

export type ExplorerAction = {
  type: string;
  skill?: string;
  prompt: number;
  slot?: number;
  edge?: number;
  vertex?: number;
  card?: number;
  cards?: number[] | null;
  choice?: string;
  target?: number;
  color?: number;
  targets?: number[];
  tokens?: number[];
  give?: number[];
  take?: number[];
  spiceLoad?: number[];
  spiceUnload?: number[];
};
export type ExplorerLocation = {
  kind: "supply" | "ship" | "harbor" | "lair" | "shoal" | "farm";
  index: number;
};
export type ExplorerMotion = {
  id: number;
  player: number;
  kind: string;
  ship: number;
  vertex: number;
  path?: number[];
  pirate?: {
    fromOwner: number;
    fromTile: number;
    toOwner: number;
    toTile: number;
  };
  lair?: {
    tile: number;
    ready: boolean;
    resolved: boolean;
    hero: number;
    dice?: number[];
  };
  chase?: {
    player: number;
    ship: number;
    sequence: number;
    die: number;
    success: boolean;
  };
  revealed?: number[];
  fishRoll?: { player: number; sequence: number; die: number; spawned: number };
  fish?: { fish: number; from: ExplorerLocation; to: ExplorerLocation }[];
  spice?: { sack: number; from: ExplorerLocation; to: ExplorerLocation }[];
  cargo?: { unit: number; from: ExplorerLocation; to: ExplorerLocation }[];
};
export type ExplorerView = {
  helperRules?: string;
  actor?: number;
  canRespond?: boolean;
  response?: {
    type: string;
    field: "tokens" | "cards" | "give" | "take";
    count: number;
    prompt: number;
  };
  actionId?: number;
  motion?: ExplorerMotion | null;
  sequence: number;
  setupPlacement?: { player: number; owner: number; kind: string };
  setupBlocked?: boolean;
  setup?: {
    start: number;
    step: number;
    harbors: number[];
    settlements: number[];
  };
  pirate?: {
    owner: number;
    tile: number;
    pending?: { stage: string; resume: string };
    lastChase?: { player: number; ship: number; die: number; success: boolean };
  };
  lairs?: {
    numberRecipe?: string;
    sites: {
      tile: number;
      number?: number;
      ready?: number;
      resolved?: number;
      hero: number;
      contributions?: number[];
      rounds?: number[][];
    }[];
    progress: number[];
    scores: number[];
    leader: number;
    left: number;
    battle?: {
      tile: number;
      player: number;
      sequence: number;
      candidates: number[];
    };
  };
  fish?: {
    lastRoll?: {
      player: number;
      sequence: number;
      die: number;
      spawned: number;
    };
    progress: number[];
    scores: number[];
    leader: number;
  };
  spice?: {
    progress: number[];
    scores: number[];
    leader: number;
    goldUse?: { player: number; sequence: number; count: number };
  };
  choices: ExplorerAction[];
  board: {
    fishing?: string;
    fishingLakes?: boolean;
    introRules?: string;
    layout?: string;
    scenario: string;
    target: number;
    unexplored: number[];
    numbersLeft: number[];
    council?: { tile: number; anchors: number[] };
    shoals?: { tile: number; number: number }[];
    farms?: {
      tile: number;
      ability: "swift" | "gold" | "pirate";
      pirateDie?: number;
    }[];
  };
  economy: {
    gold: number[];
    goldBank: number;
    bought: number;
    goldRule?: "ledger";
    goldIssued?: number;
  };
  fleet: {
    positions: number[];
    turn?: {
      player: number;
      sequence: number;
      current: number;
      open: boolean;
      fishPirate?: boolean;
      ships: {
        second?: { spent: number; wool: boolean; stopped?: boolean };
        remaining: number;
        spent: number;
        wool: boolean;
        closed: boolean;
      }[];
    };
  };
  cargo: {
    units: ExplorerLocation[];
    fish?: ExplorerLocation[];
    spice?: { origin: number; owner: number; at: ExplorerLocation }[];
    turn?: { phase: string; buildStopped?: number[] };
  };
};
export const explorerResources = ["木材", "砖块", "羊毛", "粮食", "矿石"];
export const explorerActionNames: Record<string, string> = {
  catan_helper: "使用助手",
  catan_helper_choice: "助手回应",
  catan_city: "升级城市",
  catan_wall: "建造城墙",
  catan_improvement: "城市改良",
  catan_knight_recruit: "招募骑士",
  catan_knight_activate: "激活骑士",
  catan_knight_promote: "晋升骑士",
  catan_knight_move: "移动骑士",
  catan_knight_retreat: "骑士退让",
  catan_metropolis: "放置大都会",
  catan_pillage: "降级城市",
  catan_diplomacy: "重放道路",
  catan_treason_remove: "移除骑士",
  catan_treason_place: "放置骑士",
  catan_skip_roads: "完成免费道路",
  catan_roll: "掷骰",
  catan_road: "修路",
  catan_settlement: "建村庄",
  catan_explorer_harbor: "升级港口",
  catan_explorer_ship: "造船",
  catan_explorer_unit: "招募单位",
  catan_explorer_setup: "开局放置",
  catan_explorer_pirate_place: "移动海盗",
  catan_explorer_pirate_steal: "选择偷取",
  catan_explorer_chase: "驱赶海盗",
  catan_explorer_spice_land: "派驻农场",
  catan_explorer_spice_deliver: "交付香料",
  catan_explorer_spice_gold: "农场换金币",
  catan_explorer_fish_roll: "掷捕鱼骰",
  catan_explorer_fish_load: "装载鱼群",
  catan_explorer_fish_deliver: "交付鱼群",
  catan_explorer_land: "船员登陆",
  catan_explorer_pickup: "接回船员",
  catan_explorer_resolve: "结算巢穴",
  catan_explorer_battle: "掷英雄骰",
  catan_explorer_bank: "银行兑换",
  catan_explorer_begin_move: "开始航行",
  catan_explorer_sail: "航行",
  catan_explorer_wool: "羊毛加速",
  catan_explorer_transfer: "港口装卸",
  catan_explorer_settle: "移民定居",
  catan_end: "结束回合",
};
export function explorerCanRespond(room: Room) {
  return !!(
    room.game?.catan?.explorer &&
    room.status === "playing" &&
    !room.game.finished &&
    !room.spectating &&
    room.you >= 0 &&
    room.game.catan.players[room.you] &&
    !room.game.catan.players[room.you].eliminated &&
    !room.seats[room.you]?.autoPlay
  );
}
export function explorerChoices(room: Room) {
  if (!explorerCanRespond(room)) return [];
  const x = room.game!.catan!.explorer!;
  if (x.canRespond !== undefined) {
    if (!x.canRespond || x.actor !== room.you) return [];
  } else if (room.game!.turn !== room.you) return [];
  return x.choices.filter((a) => a.prompt === x.sequence);
}
export function explorerActionKey(a: ExplorerAction) {
  return JSON.stringify(a, Object.keys(a).sort());
}
export type ExplorerPick = { room: string; action: ExplorerAction };
export function explorerSelectedAction(room: Room, pick: ExplorerPick | null) {
  if (!pick || pick.room !== room.id) return null;
  return (
    explorerChoices(room).find(
      (a) => explorerActionKey(a) === explorerActionKey(pick.action),
    ) ?? null
  );
}
export function explorerDiscardAction(room: Room, tokens: number[]) {
  const g = room.game?.catan,
    due = g?.discardDue[room.you] ?? 0,
    hand = g?.players[room.you]?.resources;
  if (
    !explorerCanRespond(room) ||
    room.game?.phase !== "catan_discard" ||
    !due ||
    !hand ||
    ![5, 8].includes(hand.length) ||
    tokens.length !== hand.length ||
    tokens.some((n, i) => !Number.isInteger(n) || n < 0 || n > hand[i]) ||
    tokens.reduce((n, x) => n + x, 0) !== due
  )
    return null;
  return {
    type: "catan_discard",
    prompt: g!.explorer!.sequence,
    tokens: [...tokens],
  };
}
export function explorerContents(g: CatanState, kind: string, index: number) {
  return (
    g.explorer?.cargo.units.flatMap((loc, id) =>
      loc.kind === kind && loc.index === index ? [id] : [],
    ) ?? []
  );
}
export function explorerFishContents(
  g: CatanState,
  kind: string,
  index: number,
) {
  return (
    g.explorer?.cargo.fish?.flatMap((loc, id) =>
      loc.kind === kind && loc.index === index ? [id] : [],
    ) ?? []
  );
}
export function explorerFreightLabel(
  units: number[],
  fish: number[],
  spice: number[] = [],
) {
  return (
    [
      units.length ? explorerCargoLabel(units) : "",
      fish.length ? `${fish.length}群鱼` : "",
      spice.length ? `${spice.length}袋香料` : "",
    ]
      .filter(Boolean)
      .join("、") || "空舱"
  );
}
export function explorerScenarioLabel(g: CatanState) {
  if (g.explorer?.spice && g.explorer.lairs) return "完整三任务";
  return g.explorer?.spice
    ? "香料与鱼群"
    : g.explorer?.fish
      ? "鱼群任务"
      : g.explorer?.lairs
        ? "海盗巢穴"
        : "初航";
}
export function explorerLairTotal(g: CatanState) {
  if (g.players.length > 4) return 8;
  return g.explorer?.board.scenario === "fish-for-catan" ? 5 : 6;
}
export function explorerFishPoint(g: CatanState, id: number) {
  const loc = g.explorer?.cargo.fish?.[id];
  if (!loc) return null;
  const p =
    loc.kind === "ship"
      ? explorerShipPosition(g, loc.index)
      : loc.kind === "harbor"
        ? g.vertices[loc.index]
        : loc.kind === "shoal"
          ? g.tiles[loc.index]
          : null;
  if (!p) return null;
  return { x: p.x, y: p.y + (loc.kind === "ship" ? -12 : 20) };
}
// Every point is in map coordinates, so zoom/scroll moves pieces and flights together.
export function explorerFishFlight(
  before: CatanState,
  after: CatanState,
  motion: ExplorerMotion,
  id: number,
) {
  const change = motion.fish?.find((f) => f.fish === id);
  if (!change) return null;
  const from = explorerFishPoint(before, id),
    to = explorerFishPoint(after, id);
  if (!from && !to) return null;
  const council = after.explorer?.board.council;
  const delivered =
    motion.kind === "catan_explorer_fish_deliver" && council
      ? after.tiles[council.tile]
      : null;
  const start = from ?? { x: to!.x, y: to!.y - 18 };
  const end =
    to ??
    (delivered
      ? { x: delivered.x, y: delivered.y }
      : { x: start.x, y: start.y - 24 });
  return {
    points: [start, end],
    appear: !from,
    retire: !to,
    delivered: !!delivered,
    width: change.to.kind === "shoal" ? 34 : 27,
  };
}
export function explorerShipPosition(g: CatanState, slot: number) {
  const positions = g.explorer?.fleet.positions,
    at = positions?.[slot];
  if (at === undefined || at < 0) return null;
  const edge = g.edges[at],
    a = g.vertices[edge.a],
    b = g.vertices[edge.b];
  const peers = positions!.flatMap((p, id) => (p === at ? [id] : []));
  const offset =
    ((peers.indexOf(slot) - (peers.length - 1) / 2) * 22 * (g.hexSize || 62)) /
    62;
  const length = Math.hypot(b.x - a.x, b.y - a.y) || 1;
  return {
    x: (a.x + b.x) / 2 - ((b.y - a.y) * offset) / length,
    y: (a.y + b.y) / 2 + ((b.x - a.x) * offset) / length,
  };
}
export function explorerTarget(
  g: CatanState,
  a: ExplorerAction,
): { kind: "edge" | "vertex" | "tile"; id: number } | null {
  if (a.choice === "skip") return null;
  if (a.type === "catan_helper" && a.edge !== undefined)
    return { kind: "edge", id: a.target! };
  if (a.type === "catan_knight_move") return { kind: "vertex", id: a.target! };
  if (a.type === "catan_explorer_fish_load") {
    const loc = g.explorer?.cargo.fish?.[a.card ?? -1];
    return loc?.kind === "shoal" ? { kind: "tile", id: loc.index } : null;
  }
  if (
    a.type === "catan_explorer_fish_deliver" ||
    a.type === "catan_explorer_spice_deliver"
  ) {
    const council = g.explorer?.board.council;
    return council ? { kind: "tile", id: council.tile } : null;
  }
  if (a.type === "catan_explorer_setup")
    return {
      kind: ["city", "harbor", "settlement"].includes(a.choice || "")
        ? "vertex"
        : "edge",
      id: a.target!,
    };
  if (
    [
      "catan_explorer_pirate_place",
      "catan_explorer_land",
      "catan_explorer_spice_land",
      "catan_explorer_pickup",
      "catan_explorer_resolve",
      "catan_explorer_battle",
    ].includes(a.type)
  )
    return { kind: "tile", id: a.target! };
  if (a.type === "catan_explorer_chase") {
    const at = g.explorer?.fleet.positions[a.target!];
    return at !== undefined && at >= 0 ? { kind: "edge", id: at } : null;
  }
  if (a.vertex !== undefined) return { kind: "vertex", id: a.vertex };
  if (a.edge !== undefined) return { kind: "edge", id: a.edge };
  if (a.type === "catan_explorer_sail" && a.targets?.length)
    return { kind: "edge", id: a.targets.at(-1)! };
  if (a.type === "catan_explorer_unit") {
    if (a.choice === "harbor") return { kind: "vertex", id: a.target! };
    const at = g.explorer?.fleet.positions[a.target!];
    if (at !== undefined && at >= 0) return { kind: "edge", id: at };
  }
  return null;
}
export function explorerIsHarbor(g: CatanState, vertex: number) {
  const v = g.vertices[vertex];
  return !!v && v.level === 2 && (!!v.harbor || !g.citiesKnights);
}

// Compact select text puts the actual choice before fees and explanation.
// The full confirmation below continues to show destination, cost and effects.
export function explorerActionOptionLabel(g: CatanState, a: ExplorerAction) {
  if (a.skill === "helper") {
    const cost =
      a.tokens ||
      (a.type === "catan_settlement" ? [1, 1, 0, 0, 0] : [0, 0, 0, 1, 2]);
    const payment = cost
      .flatMap((n, i) => (n ? [`${explorerResources[i]}${n}`] : []))
      .join("＋");
    const detail =
      a.type === "catan_explorer_ship"
        ? `船${(a.slot! % 3) + 1}`
        : ["catan_settlement", "catan_explorer_harbor", "catan_city"].includes(
              a.type,
            )
          ? `归还${explorerUnitLabel(a.card!)}`
          : "修路";
    return `${detail} · ${payment}`;
  }

  if (a.type !== "catan_explorer_unit") return explorerActionDescription(g, a);
  const returned = explorerFreightLabel(
    a.cards || [],
    a.targets || [],
    a.spiceUnload || [],
  );
  const hasReturn =
    (a.cards?.length || 0) +
      (a.targets?.length || 0) +
      (a.spiceUnload?.length || 0) >
    0;
  return `${explorerUnitLabel(a.card ?? 0)}${hasReturn ? ` · 归还${returned}` : ""}`;
}
export function explorerActionDescription(g: CatanState, a: ExplorerAction) {
  if (a.skill === "helper") {
    const cost =
      a.tokens ||
      (a.type === "catan_settlement" ? [1, 1, 0, 0, 0] : [0, 0, 0, 1, 2]);
    const payment = cost
      .flatMap((n, i) => (n ? [`${explorerResources[i]}×${n}`] : []))
      .join("、");
    const unit =
      a.card !== undefined &&
      ["catan_settlement", "catan_explorer_harbor", "catan_city"].includes(
        a.type,
      )
        ? `归还${explorerUnitLabel(a.card)}（${g.explorer?.cargo.units[a.card]?.kind === "ship" ? `船${(g.explorer.cargo.units[a.card].index % 3) + 1}` : `港口${(g.explorer?.cargo.units[a.card]?.index ?? 0) + 1}`}），`
        : "";
    const recycled =
      a.type === "catan_explorer_ship" &&
      (g.explorer?.fleet.positions[a.slot!] ?? -1) >= 0;
    return `助手${explorerActionNames[a.type]}：${unit}支付${payment}。${a.slot !== undefined ? `建造船${(a.slot % 3) + 1}。` : ""}${recycled ? "将拆回原船及全部货物，再建造新船。" : ""}完成后翻面或交换助手。`;
  }
  if (a.type === "catan_helper" && a.edge !== undefined)
    return `免费将末端道路${a.edge + 1}迁移到道路${a.target! + 1}，随后翻面或交换助手。`;

  const ship = `船${((a.slot ?? 0) % 3) + 1}`,
    vertex = `位置${(a.vertex ?? 0) + 1}`;
  switch (a.type) {
    case "catan_explorer_setup":
      return `${(g.explorer?.setupPlacement?.owner ?? 0) < 0 ? "为中立方" : "为自己"}免费放置${{ city: "城市", harbor: "港口", settlement: "村庄", road: "道路", ship: "载移民的船" }[a.choice!] || "棋子"}，确认后进入下一步。`;
    case "catan_explorer_pirate_place":
      return `把你的海盗放到海格${a.target! + 1}；原海盗被替换，有合适对手时再选择偷取对象。`;
    case "catan_explorer_pirate_steal":
      return a.choice === "skip"
        ? `放弃偷取玩家${a.target! + 1}的金币。`
        : `从玩家${a.target! + 1}随机偷取1张${g.citiesKnights ? "资源或商品" : "资源"}；对方空手时偷取1金币。`;
    case "catan_explorer_chase": {
      const player = Math.floor(a.target! / 3),
        bonus = explorerFarmAbilities(g, player).pirate;
      return `船${(a.target! % 3) + 1}掷骰驱赶海盗，${[...bonus, 6].join("、")}点成功；本船本回合限一次，不消耗移动点。`;
    }
    case "catan_explorer_spice_land": {
      const farm = g.explorer?.board.farms?.find((f) => f.tile === a.target);
      return `${ship}向农场${a.target! + 1}永久派驻1名船员，领取1袋香料（占1格）。船员不能召回，每人每座农场限一次；${farm ? explorerFarmDescription(farm) : "获得此农场能力"}。不消耗移动点。`;
    }
    case "catan_explorer_spice_deliver":
      return `${ship}向议会岛交付1袋香料，推进香料任务并归还香料袋；农场能力保留，不消耗移动点。`;
    case "catan_explorer_spice_gold":
      return `支付1${catanCardNames[a.card!]}，获得1金币；每座已派驻金币农场每行动阶段限一次，与2金币购买资源的额度独立。`;
    case "catan_explorer_fish_roll":
      return "本航行阶段可掷一次捕鱼骰；点数对应已探索且未被海盗封锁的空渔场时，从供应放入一群鱼。没有合适渔场或供应耗尽时也会用掉这次掷骰。";
    case "catan_explorer_fish_load":
      return `${ship}从相邻渔场装入一群鱼，占满2格船舱，不消耗移动点。`;
    case "catan_explorer_fish_deliver":
      return `${ship}在议会岛锚点交付一群鱼，推进鱼群任务并把鱼群归还供应，不消耗移动点。`;
    case "catan_explorer_land":
      return `${ship}向巢穴${a.target! + 1}派出${a.cards?.length || 0}名船员，不消耗移动点。凑满3人后，结束航行时结算。`;
    case "catan_explorer_pickup":
      return `${ship}从金矿${a.target! + 1}接回${a.cards?.length || 0}名己方船员，不消耗移动点。`;
    case "catan_explorer_resolve":
      return `结算巢穴${a.target! + 1}：参与者各得2金币并推进任务，随后决出英雄。`;
    case "catan_explorer_battle":
      return "参与者掷骰加己方船员数，比总点数；同分比船员数，仍相同者重掷。英雄额外推进一步并归还一名船员。";
    case "catan_road":
      return g.freeRoads > 0
        ? "免费修建这条道路，不支付木材或砖块；不能用此机会造船。"
        : "支付1木材、1砖块。";
    case "catan_skip_roads":
      return "已没有合法的免费道路位置，完成道路建设并恢复行动。";
    case "catan_city":
      return "支付2粮食、3矿石，将村庄升级为城市，增加1分；横置城市必须优先修复。";
    case "catan_wall":
      return "支付2砖块，为城市建造城墙，七点弃牌上限增加2张；每人最多三座。";
    case "catan_knight_recruit":
      return `支付1羊毛、1矿石，在交点${a.vertex! + 1}招募一级骑士（未激活）。`;
    case "catan_knight_activate":
      return `支付1粮食，激活交点${a.vertex! + 1}的骑士；本行动阶段不能再让它移动。`;
    case "catan_knight_promote":
      return `支付1羊毛、1矿石，将交点${a.vertex! + 1}的骑士晋升一级。`;
    case "catan_knight_move":
      return `将骑士从交点${a.vertex! + 1}移到${a.target! + 1}，随后转为未激活；较弱敌方骑士须退让。`;
    case "catan_knight_retreat":
      return `将被驱逐的骑士退到交点${a.vertex! + 1}，保留原激活状态。`;
    case "catan_metropolis":
      return `在城市${a.vertex! + 1}放置大都会，额外获得2分。`;
    case "catan_pillage":
      return `城市${a.vertex! + 1}横置，降为村庄生产并失去1分；城墙归还，港口不受此劫掠。`;
    case "catan_diplomacy":
      return a.choice === "skip"
        ? "放弃免费重建道路。"
        : `免费将道路重放到位置${a.edge! + 1}。`;
    case "catan_treason_remove":
      return `因叛变移除交点${a.vertex! + 1}的骑士；对方随后选择放置。`;
    case "catan_treason_place":
      return a.choice === "skip"
        ? "放弃免费放置骑士。"
        : `在交点${a.vertex! + 1}放置${a.color}级骑士，保留被移除骑士的激活状态。`;
    case "catan_settlement":
      return "支付木、砖、羊、粮各1，建造村庄，获得1分。";
    case "catan_explorer_harbor":
      return "支付2粮食、2矿石，村庄升级为港口，增加1分；每块相邻地形仍生产1资源。";
    case "catan_explorer_ship":
      return `支付1木材、1羊毛，建造${ship}。${(g.explorer?.fleet.positions[a.slot!] ?? -1) >= 0 ? "将拆回原船及全部货物，再建造新船。" : ""}`;
    case "catan_explorer_unit":
      return `支付${(a.card ?? 0) % 11 < 2 ? "木、砖、羊、粮各1" : "1羊毛、1矿石"}，在${a.choice === "ship" ? `船${(a.target! % 3) + 1}` : `港口${a.target! + 1}`}放置${explorerUnitLabel(a.card ?? 0)}，占${(a.card ?? 0) % 11 < 2 ? 2 : 1}格。${a.cards?.length ? `先归还${explorerCargoLabel(a.cards)}。` : ""}${a.targets?.length ? "先归还1群鱼，不推进鱼群任务。" : ""}${a.spiceUnload?.length ? `先归还${a.spiceUnload.length}袋香料，不推进香料任务；农场能力保留，不能再次领取这些香料。` : ""}${(a.cards?.length || 0) + (a.targets?.length || 0) + (a.spiceUnload?.length || 0) > 1 ? "本站补充规则：满舱时，只归还建造移民所需的两件小货物。" : ""}`;
    case "catan_explorer_bank": {
      const rate =
        a.target === -1
          ? 3
          : (g.players[g.explorer?.actor ?? -1]?.rates?.[a.color!] ??
            (a.color! >= 5 ? 4 : 3));
      return a.color === -1
        ? `支付2金币，领取1${catanCardNames[a.target!]}（本行动阶段最多2次）。`
        : `支付${rate}${catanCardNames[a.color!]}，领取1${a.target === -1 ? "金币" : catanCardNames[a.target!]}。`;
    }
    case "catan_explorer_sail": {
      const target = g.edges[a.targets!.at(-1)!];
      const fog = g.tiles.some(
        (t) =>
          t.resource === 8 &&
          (t.vertices.includes(target.a) || t.vertices.includes(target.b)),
      );
      const current = g.explorer?.fleet.turn?.current ?? -1;
      return `${ship}航行${a.targets!.length}步。${fog ? "发现迷雾后本船停止移动。" : ""}${current >= 0 && current !== a.slot ? "切换船只后，上一艘船不能继续移动。" : ""}`;
    }
    case "catan_explorer_wool":
      return `支付1羊毛，为${ship}增加2点移动，每船每回合限一次。`;
    case "catan_explorer_transfer": {
      const load = explorerFreightLabel(
          a.give || [],
          a.cards || [],
          a.spiceLoad || [],
        ),
        unload = explorerFreightLabel(
          a.take || [],
          a.targets || [],
          a.spiceUnload || [],
        );
      return `${ship}在港口${a.vertex! + 1}${[load !== "空舱" ? `装入${load}` : "", unload !== "空舱" ? `卸下${unload}` : ""].filter(Boolean).join("并")}，不消耗移动点。`;
    }
    case "catan_explorer_settle":
      return `${ship}的移民在${vertex}定居，获得1分；归还船和移民，不再支付资源。`;
    case "catan_explorer_begin_move":
      return "结束本回合交易与建设，进入航行阶段；共用原回合剩余时间。";
    case "catan_end":
      return g.explorer?.lairs
        ? "结束所有船只的移动；本回合攻陷的巢穴会先结算，再交给下一位玩家。"
        : "结束所有船只的移动，交给下一位玩家。";
    default:
      return g.eventDeck
        ? "翻开一张事件牌，只取生产点数，忽略事件文字；保留金币补偿和7点规则。"
        : "掷骰并按点数生产资源。";
  }
}

// Consecutive authoritative updates only: initial/reconnected snapshots never replay.
export function explorerMotionBetween(before: Room, after: Room) {
  const old = before.game?.catan?.explorer,
    next = after.game?.catan?.explorer;
  if (
    !old ||
    !next ||
    before.id !== after.id ||
    before.you !== after.you ||
    !!before.spectating !== !!after.spectating ||
    before.status !== "playing" ||
    !["playing", "finished"].includes(after.status) ||
    after.version !== before.version + 1 ||
    next.actionId !== (old.actionId ?? 0) + 1 ||
    next.motion?.id !== next.actionId
  )
    return null;
  return next.motion ?? null;
}
export function explorerMotionPath(
  before: CatanState,
  after: CatanState,
  motion: ExplorerMotion,
) {
  if (!motion.path?.length) return [];
  const start = explorerShipPosition(before, motion.ship),
    end = explorerShipPosition(after, motion.ship);
  if (!start || !end) return [];
  const middle = motion.path.slice(1, -1).map((id) => {
    const e = after.edges[id];
    if (!e) return null;
    const a = after.vertices[e.a],
      b = after.vertices[e.b];
    return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
  });
  return middle.some((p) => !p)
    ? []
    : [start, ...(middle as { x: number; y: number }[]), end];
}

export function explorerPathPoint(
  points: { x: number; y: number }[],
  progress: number,
) {
  if (points.length < 2) return points[0] ?? { x: 0, y: 0 };
  const lengths = points
    .slice(1)
    .map((p, i) => Math.hypot(p.x - points[i].x, p.y - points[i].y));
  let distance =
    lengths.reduce((a, b) => a + b, 0) * Math.max(0, Math.min(1, progress));
  for (let i = 0; i < lengths.length; i++) {
    if (distance <= lengths[i] && lengths[i] > 0) {
      const t = distance / lengths[i];
      return {
        x: points[i].x + (points[i + 1].x - points[i].x) * t,
        y: points[i].y + (points[i + 1].y - points[i].y) * t,
      };
    }
    distance -= lengths[i];
  }
  return points[points.length - 1];
}

export function explorerUnitLabel(id: number) {
  return id % 11 < 2 ? `移民${(id % 11) + 1}` : `船员${(id % 11) - 1}`;
}
export function explorerCargoLabel(ids: number[]) {
  const settlers = ids.filter((id) => id % 11 < 2).length,
    crew = ids.length - settlers;
  return (
    [settlers ? `${settlers}枚移民` : "", crew ? `${crew}名船员` : ""]
      .filter(Boolean)
      .join("、") || "空舱"
  );
}
export function explorerPhaseLabel(phase: string) {
  if (phase === "catan_helper") return "助手回应";
  return (
    (
      {
        catan_explorer_setup: "开局放置",
        catan_roll: "掷骰生产",
        catan_discard: "所有人同时弃牌",
        catan_fish_replace: "选择鱼筹码",
        catan_turn: "交易与建设",
        catan_roads: "免费修建道路",
        catan_aqueduct: "引水渠补偿",
        catan_metropolis: "大都会选址",
        catan_pillage: "蛮族劫掠",
        catan_defender_reward: "防御者奖励",
        catan_progress_discard: "进步牌超限",
        catan_progress_end: "航行前整理进步牌",
        catan_knight_retreat: "骑士退让",
        catan_guild_dues: "行会征费",
        catan_commercial_harbor: "商业港交换",
        catan_diplomacy: "外交迁路",
        catan_espionage: "间谍选牌",
        catan_sabotage: "破坏弃牌",
        catan_wedding: "婚礼赠牌",
        catan_treason_remove: "叛变移除骑士",
        catan_treason_place: "叛变安放骑士",
        catan_explorer_move: "船只航行",
        catan_explorer_pirate_place: "放置海盗",
        catan_explorer_pirate_steal: "海盗偷取",
        catan_explorer_resolve: "结算攻陷巢穴",
        catan_explorer_battle: "决出巢穴英雄",
        finished: "本局已结束",
      } as Record<string, string>
    )[phase] || "探险行动"
  );
}

// Anchors use the exact same map-space row spacing as static cargo pieces.
export function explorerCargoPoint(
  g: CatanState,
  unit: number,
  loc: ExplorerLocation,
) {
  const p =
    loc.kind === "ship"
      ? explorerShipPosition(g, loc.index)
      : loc.kind === "harbor"
        ? g.vertices[loc.index]
        : loc.kind === "lair" || loc.kind === "farm"
          ? g.tiles[loc.index]
          : null;
  if (!p) return null;
  const peers = explorerContents(g, loc.kind, loc.index),
    index = peers.indexOf(unit);
  if (index < 0) return null;
  return {
    x:
      p.x +
      (index -
        (peers.length +
          (["ship", "harbor"].includes(loc.kind)
            ? explorerSpiceContents(g, loc.kind, loc.index).length
            : 0) -
          1) /
          2) *
        (["lair", "farm"].includes(loc.kind) ? 18 : 16),
    y:
      p.y +
      (loc.kind === "ship"
        ? -12
        : ["lair", "farm"].includes(loc.kind)
          ? 34
          : 20),
  };
}

export function explorerSpiceContents(
  g: CatanState,
  kind: string,
  index: number,
) {
  return (
    g.explorer?.cargo.spice?.flatMap((s, id) =>
      s.at.kind === kind && s.at.index === index ? [id] : [],
    ) ?? []
  );
}
export function explorerFarmAbilities(g: CatanState, player: number) {
  const claimed = new Set(
    g.explorer?.cargo.spice
      ?.filter((s) => s.owner === player)
      .map((s) => s.origin),
  );
  const farms =
    g.explorer?.board.farms?.filter((f) => claimed.has(f.tile)) ?? [];
  return {
    swift: farms.filter((f) => f.ability === "swift").length,
    gold: farms.filter((f) => f.ability === "gold").length,
    pirate: farms
      .filter((f) => f.ability === "pirate")
      .map((f) => f.pirateDie!)
      .sort(),
    count: farms.length,
  };
}
export function explorerFarmDescription(farm: {
  ability: string;
  pirateDie?: number;
}) {
  return farm.ability === "swift"
    ? "所有己方船航速＋1，已停止的船不重开"
    : farm.ability === "gold"
      ? "每行动阶段可用1资源换1金币"
      : `驱赶海盗掷出${farm.pirateDie}也成功`;
}
export function explorerSpicePoint(g: CatanState, id: number) {
  const loc = g.explorer?.cargo.spice?.[id]?.at;
  if (!loc) return null;
  const p =
    loc.kind === "ship"
      ? explorerShipPosition(g, loc.index)
      : loc.kind === "harbor"
        ? g.vertices[loc.index]
        : loc.kind === "farm"
          ? g.tiles[loc.index]
          : null;
  if (!p) return null;
  const sacks = explorerSpiceContents(g, loc.kind, loc.index),
    units =
      loc.kind === "farm" ? 0 : explorerContents(g, loc.kind, loc.index).length;
  return {
    x: p.x + (units + sacks.indexOf(id) - (units + sacks.length - 1) / 2) * 16,
    y: p.y + (loc.kind === "ship" ? -12 : loc.kind === "farm" ? 6 : 20),
  };
}
export function explorerSpiceFlight(
  before: CatanState,
  after: CatanState,
  motion: ExplorerMotion,
  id: number,
) {
  if (!motion.spice?.some((s) => s.sack === id)) return null;
  const from = explorerSpicePoint(before, id),
    to = explorerSpicePoint(after, id);
  if (!from && !to) return null;
  const council = after.explorer?.board.council;
  const delivered =
    motion.kind === "catan_explorer_spice_deliver" && council
      ? after.tiles[council.tile]
      : null;
  const start = from ?? { x: to!.x, y: to!.y - 18 };
  return {
    points: [
      start,
      to ??
        (delivered
          ? { x: delivered.x, y: delivered.y }
          : { x: start.x, y: start.y - 24 }),
    ],
    appear: !from,
    retire: !to,
    delivered: !!delivered,
  };
}

export function explorerSupplyLimits(g: CatanState) {
  return g.players.length > 4
    ? { resource: 24, gold: 172 }
    : { resource: 19, gold: 148 };
}

export function explorerCanOffer(room: Room) {
  const game = room.game;
  return (
    !!game?.catan?.explorer &&
    explorerCanRespond(room) &&
    game.turn === room.you &&
    game.phase === "catan_turn" &&
    !game.catan.paired?.second
  );
}
