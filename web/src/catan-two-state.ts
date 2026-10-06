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
export function twoReturnValid(hand: number[], give: number[]) {
  return (
    give.length === 5 &&
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

export type TwoRetreatSelection = { tile: number | null } | null;

export function twoChoiceName(
  g: CatanState,
  c: { edge: number; vertex: number },
) {
  return c.vertex >= 0
    ? "村庄"
    : g.rivers?.map.bridges.includes(c.edge)
      ? "桥梁"
      : "道路";
}

export function twoRetreatTargets(room: Room): number[] {
  const g = room.game?.catan,
    q = g?.two;
  if (
    !g ||
    !q ||
    room.status !== "playing" ||
    room.game?.finished ||
    room.spectating ||
    room.you < 0 ||
    !g.players[room.you] ||
    g.players[room.you].eliminated ||
    !q.tokenWindow ||
    q.spent ||
    q.tokens[room.you] < (q.cost || 1)
  )
    return [];
  if (g.caravans)
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
