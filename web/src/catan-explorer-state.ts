import type { CatanState, Room } from "./types";

export type ExplorerAction = {
  type: string;
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
  give?: number[];
  take?: number[];
};
export type ExplorerLocation = {
  kind: "supply" | "ship" | "harbor";
  index: number;
};
export type ExplorerMotion = {
  id: number;
  player: number;
  kind: string;
  ship: number;
  vertex: number;
  path?: number[];
  revealed?: number[];
  cargo?: { unit: number; from: ExplorerLocation; to: ExplorerLocation }[];
};
export type ExplorerView = {
  actionId?: number;
  motion?: ExplorerMotion | null;
  sequence: number;
  choices: ExplorerAction[];
  board: {
    scenario: string;
    target: number;
    unexplored: number[];
    numbersLeft: number[];
  };
  economy: { gold: number[]; goldBank: number; bought: number };
  fleet: {
    positions: number[];
    turn?: {
      player: number;
      sequence: number;
      current: number;
      open: boolean;
      ships: {
        remaining: number;
        spent: number;
        wool: boolean;
        closed: boolean;
      }[];
    };
  };
  cargo: {
    units: ExplorerLocation[];
    turn?: { phase: string; buildStopped?: number[] };
  };
};
export const explorerResources = ["木材", "砖块", "羊毛", "粮食", "矿石"];
export const explorerActionNames: Record<string, string> = {
  catan_roll: "掷骰",
  catan_road: "修路",
  catan_settlement: "建村庄",
  catan_explorer_harbor: "升级港口",
  catan_explorer_ship: "造船",
  catan_explorer_unit: "招募移民",
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
  if (!explorerCanRespond(room) || room.game!.turn !== room.you) return [];
  const x = room.game!.catan!.explorer!;
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
    tokens.length !== 5 ||
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
): { kind: "edge" | "vertex"; id: number } | null {
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
export function explorerActionDescription(g: CatanState, a: ExplorerAction) {
  const ship = `船${((a.slot ?? 0) % 3) + 1}`,
    vertex = `位置${(a.vertex ?? 0) + 1}`;
  switch (a.type) {
    case "catan_road":
      return "支付1木材、1砖块。";
    case "catan_settlement":
      return "支付木、砖、羊、粮各1，建造村庄，获得1分。";
    case "catan_explorer_harbor":
      return "支付2粮食、2矿石，村庄升级为港口，增加1分；每块相邻地形仍生产1资源。";
    case "catan_explorer_ship":
      return `支付1木材、1羊毛，建造${ship}。${(g.explorer?.fleet.positions[a.slot!] ?? -1) >= 0 ? "将拆回原船及全部货物，再建造新船。" : ""}`;
    case "catan_explorer_unit":
      return `支付木、砖、羊、粮各1，在${a.choice === "ship" ? `船${(a.target! % 3) + 1}` : `港口${a.target! + 1}`}放置移民，占2格。${a.cards?.length ? "会先归还该舱位的原移民。" : ""}`;
    case "catan_explorer_bank":
      return a.color === -1
        ? `支付2金币，领取1${explorerResources[a.target!]}（本回合最多2次）。`
        : `支付3${explorerResources[a.color!]}，领取1${a.target === -1 ? "金币" : explorerResources[a.target!]}。`;
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
    case "catan_explorer_transfer":
      return `${ship}在港口${a.vertex! + 1}${a.give?.length ? "装入移民" : ""}${a.give?.length && a.take?.length ? "并" : ""}${a.take?.length ? "卸下移民" : ""}，不消耗移动点。`;
    case "catan_explorer_settle":
      return `${ship}的移民在${vertex}定居，获得1分；归还船和移民，不再支付资源。`;
    case "catan_explorer_begin_move":
      return "结束本回合交易与建设，进入航行阶段；共用原回合剩余时间。";
    case "catan_end":
      return "结束所有船只的移动，交给下一位玩家。";
    default:
      return "掷骰并按点数生产资源。";
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
