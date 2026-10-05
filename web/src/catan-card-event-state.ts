import type { Room, CatanState, CatanRevealedEvent } from "./types";

export const catanEventNames: Record<string, string> = {
  beautiful_day: "美好的一天",
  calm_seas: "风平浪静",
  conflict: "冲突",
  earthquake: "地震",
  epidemic: "瘟疫",
  good_neighbors: "好邻居",
  helpful_neighbor: "援助邻居",
  new_year: "新年",
  plentiful_year: "丰收年",
  robber_attacks: "强盗袭击",
  robber_flees: "强盗逃跑",
  tournament: "比武大会",
  trade_advantage: "贸易优势",
};

export function catanCardEventActor(room: Room) {
  const g = room.game?.catan;
  return !!(
    g?.cardEvent &&
    room.status === "playing" &&
    !room.game?.finished &&
    room.game?.phase === "catan_card_event" &&
    !room.spectating &&
    room.you >= 0 &&
    g.players[room.you] &&
    !g.players[room.you].eliminated &&
    g.cardEvent.players[0] === room.you
  );
}

export function catanCardEventMapMode(room: Room) {
  if (!catanCardEventActor(room)) return "";
  const kind = room.game!.catan!.cardEvent!.kind;
  return kind === "earthquake" || kind === "robber_flees" ? kind : "";
}

export type CatanEventSelection = {
  color: number | null;
  target: number | null;
  map: { type: string; id: number } | null;
  skip?: boolean;
};

// Only construct actions from this viewer's server-provided legal choices.
// Invalid/stale selections must not become a confirmed action after a refresh.
export function catanCardEventAction(
  room: Room,
  selection: CatanEventSelection,
): Record<string, unknown> | null {
  if (!catanCardEventActor(room)) return null;
  const g = room.game!.catan!,
    q = g.cardEvent!,
    { color, target, map, skip } = selection;
  if (skip)
    return q.kind === "conflict" && q.canSkip
      ? { type: "catan_event_skip" }
      : null;
  if (q.kind === "earthquake")
    return map?.type === "earthquake" &&
      g.legal.earthquakeRoads?.includes(map.id)
      ? { type: "catan_earthquake", edge: map.id }
      : null;
  if (q.kind === "robber_flees")
    return map?.type === "robber_flees" && g.legal.fleeDeserts?.includes(map.id)
      ? { type: "catan_robber_flees", tile: map.id }
      : null;
  if (["conflict", "trade_advantage"].includes(q.kind))
    return target !== null && g.legal.eventTargets?.includes(target)
      ? { type: "catan_event_steal", target }
      : null;
  if (["plentiful_year", "calm_seas", "tournament"].includes(q.kind)) {
    if (
      color === null ||
      !Number.isInteger(color) ||
      color < 0 ||
      color >= 5 ||
      !g.legal.eventResources?.includes(color) ||
      !(g.bank[color] > 0)
    )
      return null;
    const take = Array(5).fill(0);
    take[color] = 1;
    return { type: "catan_event_resource", take };
  }
  if (q.kind === "good_neighbors" || q.kind === "helpful_neighbor") {
    if (
      color === null ||
      !Number.isInteger(color) ||
      color < 0 ||
      color >= g.bank.length ||
      !g.legal.eventGifts?.includes(color) ||
      !((g.players[room.you].resources?.[color] || 0) > 0)
    )
      return null;
    const give = Array(g.bank.length).fill(0);
    give[color] = 1;
    if (q.kind === "helpful_neighbor")
      return target !== null && g.legal.eventTargets?.includes(target)
        ? { type: "catan_event_gift", give, target }
        : null;
    return { type: "catan_event_gift", give };
  }
  return null;
}

// The response queue is transient. Keep the public face throughout city-die
// responses, production, paired turns and refreshes, until the next roll.
export function catanRevealedEvent(
  g: CatanState,
): CatanRevealedEvent | undefined {
  if (g.cardEvent)
    return {
      kind: g.cardEvent.kind,
      production: g.cardEvent.production,
      red: g.cardEvent.red,
      face: g.cardEvent.face,
      rollId: g.rollId,
      productionStarted: false,
    };
  return g.revealedEvent?.rollId === g.rollId ? g.revealedEvent : undefined;
}
