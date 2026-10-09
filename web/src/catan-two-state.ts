import type { CatanState, CatanTwoChoice, Room } from "./types";

export type TwoSelection = {
  sequence: number;
  edge: number;
  vertex: number;
  owner: number | null;
};

export function twoResponder(room: Room) {
  const q = room.game?.catan?.two;
  return room.status === "playing" &&
    !room.game?.finished &&
    q &&
    (q.pending || q.trade) &&
    q.actor >= 0
    ? q.actor
    : undefined;
}
export function twoCanRespond(room: Room) {
  return (
    !room.spectating &&
    room.you >= 0 &&
    twoResponder(room) === room.you &&
    !!room.game?.catan?.two?.canAct &&
    !room.seats?.[room.you]?.autoPlay &&
    !room.game.catan.players[room.you]?.eliminated
  );
}
export function twoChoices(g: CatanState, selected: TwoSelection | null) {
  if (!selected || !g.two?.pending || selected.sequence !== g.two.sequence)
    return [];
  return (g.two.choices || []).filter(
    (c) => c.edge === selected.edge && c.vertex === selected.vertex,
  );
}
export function twoPick(
  g: CatanState,
  edge: number,
  vertex: number,
): TwoSelection | null {
  const choices = (g.two?.choices || []).filter(
    (c) => c.edge === edge && c.vertex === vertex,
  );
  return g.two?.pending && choices.length
    ? {
        sequence: g.two.sequence,
        edge,
        vertex,
        owner: choices.length === 1 ? choices[0].owner : null,
      }
    : null;
}
export function twoSelected(
  g: CatanState,
  selected: TwoSelection | null,
): CatanTwoChoice | undefined {
  return twoChoices(g, selected).find((c) => c.owner === selected?.owner);
}
export function twoReturnValid(hand: number[], give: number[], mixed = true) {
  return (
    [5, 8].includes(hand.length) &&
    give.length === hand.length &&
    (mixed || give.slice(5).every((n) => n === 0)) &&
    give.every(
      (n, i) => Number.isInteger(n) && n >= 0 && n <= (hand[i] || 0),
    ) &&
    give.reduce((a, b) => a + b, 0) === 2
  );
}
export const twoNeutralName = (owner: number) => `中立势力 ${-owner - 1}`;

// Compare public board state so observers see the same confirmed placement.
// Never replay a placement after a reconnect, rematch or viewer change.
export function twoNeutralAdded(
  before: Room,
  after: Room,
): CatanTwoChoice | null {
  const a = before.game?.catan,
    b = after.game?.catan;
  if (
    !a?.two ||
    !b?.two ||
    before.id !== after.id ||
    before.you !== after.you ||
    !!before.spectating !== !!after.spectating ||
    before.status !== "playing" ||
    !["playing", "finished"].includes(after.status) ||
    after.version !== before.version + 1 ||
    a.rollId !== b.rollId ||
    a.two.sequence !== b.two.sequence ||
    !a.two.pending ||
    b.two.pending ||
    a.setupStep < (a.setupLimit ?? a.players.length * 2) ||
    b.setupStep < (b.setupLimit ?? b.players.length * 2) ||
    a.edges.length !== b.edges.length ||
    a.vertices.length !== b.vertices.length
  )
    return null;
  const added: CatanTwoChoice[] = [];
  for (const e of b.edges) {
    const prior = a.edges[e.id];
    if (!prior || prior.id !== e.id) return null;
    if (prior.owner === e.owner) continue;
    if (prior.owner !== -1 || ![-2, -3].includes(e.owner)) return null;
    added.push({ owner: e.owner, edge: e.id, vertex: -1 });
  }
  for (const v of b.vertices) {
    const prior = a.vertices[v.id];
    if (!prior || prior.id !== v.id) return null;
    if (prior.owner === v.owner && prior.level === v.level) continue;
    if (prior.level !== 0 || v.level !== 1 || ![-2, -3].includes(v.owner))
      return null;
    added.push({ owner: v.owner, edge: -1, vertex: v.id });
  }
  return added.length === 1 ? added[0] : null;
}

export type TwoRetreatSelection = {
  tile: number | null;
  edge?: number | null;
  piece?: number | null;
} | null;

export function twoRetreatCost(room: Room): number {
  const g = room.game?.catan;
  return g?.transport ? (g.two?.retreatCost ?? 1) : (g?.two?.cost ?? 1);
}

function twoMayRetreat(room: Room): boolean {
  const g = room.game?.catan,
    q = g?.two;
  return !!(
    g &&
    q &&
    room.status === "playing" &&
    !room.game?.finished &&
    !room.spectating &&
    room.you >= 0 &&
    room.game?.turn === room.you &&
    g.players[room.you] &&
    !g.players[room.you].eliminated &&
    !room.seats?.[room.you]?.autoPlay &&
    q.tokenWindow &&
    !q.spent &&
    q.tokens[room.you] >= twoRetreatCost(room)
  );
}

export function twoRetreatEdges(room: Room): number[] {
  const g = room.game?.catan;
  if (!g?.transport || !twoMayRetreat(room)) return [];
  return [...new Set(g.two?.retreatEdges || [])].filter(
    (id) =>
      g.edges[id]?.id === id &&
      g.edges[id].owner === -1 &&
      !g.transport!.state.barbarians.includes(id),
  );
}

export function twoRetreatAction(
  room: Room,
  selected: TwoRetreatSelection,
): Record<string, unknown> | null {
  if (!selected) return null;
  const g = room.game?.catan;
  if (g?.attack) {
    const { piece, tile } = selected;
    return twoMayRetreat(room) &&
      piece != null &&
      tile != null &&
      !!g.two?.attackMoves?.[piece]?.includes(tile)
      ? { type: "catan_two_robber", card: piece, tile }
      : null;
  }
  const t = g?.transport;
  if (t) {
    const { edge, piece } = selected;
    return edge != null &&
      piece != null &&
      Number.isInteger(piece) &&
      piece >= 0 &&
      piece < t.state.barbarians.length &&
      twoRetreatEdges(room).includes(edge)
      ? { type: "catan_two_robber", card: piece, edge }
      : null;
  }
  return selected.tile != null &&
    twoRetreatTargets(room).includes(selected.tile)
    ? { type: "catan_two_robber", tile: selected.tile }
    : null;
}

export function twoChoiceName(
  g: CatanState,
  c: { edge: number; vertex: number },
) {
  if (g.two?.buildingRoadKnight)
    return g.two.pending?.kind === "knight_promote"
      ? "道路骑士升级"
      : "一级道路骑士";
  return c.vertex >= 0
    ? g.two?.pending?.kind === "knight"
      ? "一级骑士"
      : g.two?.pending?.kind === "knight_promote"
        ? "骑士升级"
        : "村庄"
    : g.rivers?.map.bridges.includes(c.edge)
      ? "桥梁"
      : g.two?.buildingShip
        ? "船只"
        : "道路";
}

export function twoRetreatTargets(room: Room): number[] {
  const g = room.game?.catan,
    q = g?.two;
  if (!g || !q || g.transport || g.attack || !twoMayRetreat(room)) return [];
  if (
    (g.caravans && !g.rivers) ||
    (q.seafarers === "wire-board-two-seafarers-v1" &&
      !g.tiles.some((t) => t.resource === 5))
  )
    return g.robber >= 0 && q.retreatTiles?.includes(-1) ? [-1] : [];
  // Respect server choices; only old base saves may fall back to the desert.
  const targets =
    q.retreatTiles ??
    (g.rivers ? [] : g.tiles.filter((t) => t.resource === 5).map((t) => t.id));
  return [...new Set(targets)].filter(
    (id) =>
      g.tiles[id]?.id === id &&
      id !== g.robber &&
      (g.rivers
        ? g.rivers.map.swamps.includes(id)
        : g.tiles[id].resource === 5),
  );
}
