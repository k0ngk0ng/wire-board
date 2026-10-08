import type { CatanState, Room } from "./types";

export const progressChoiceLabels: Record<string, string> = {
  guild_dues: "行会征费 · 选取手牌",
  commercial_harbor: "商业港 · 交换商品",
  espionage: "间谍 · 选取进步牌",
  wedding: "婚礼 · 赠送手牌",
  sabotage: "破坏 · 弃置手牌",
  diplomacy: "外交 · 重放路线",
  treason_remove: "叛变 · 移除骑士",
  treason_place: "叛变 · 放置骑士",
};
export const progressMapChoices = [
  "diplomacy",
  "treason_remove",
  "treason_place",
];
const sum = (a: number[]) => a.reduce((s, n) => s + n, 0);
export function treasonStrengths(g: CatanState, player: number) {
  const k = g.citiesKnights;
  return [1, 2, 3].filter(
    (n) =>
      n <= (k?.pending?.knight?.strength || 0) &&
      (k?.knights.filter((v) => v.owner === player && v.strength === n)
        .length || 0) < 2,
  );
}
export function progressChoiceHand(g: CatanState, player: number) {
  const q = g.citiesKnights?.pending;
  if (!q || q.players[0] !== player) return [];
  return (
    (q.kind === "guild_dues" ? q.resources : g.players[player]?.resources) || []
  );
}
export function progressChoiceDue(g: CatanState, player: number) {
  const count = sum(progressChoiceHand(g, player));
  return g.citiesKnights?.pending?.kind === "sabotage"
    ? Math.floor(count / 2)
    : Math.min(2, count);
}
export type ProgressChoiceSelection = {
  bundle: number[];
  color: number | null;
  card: number | null;
  map: { type: string; id: number } | null;
  skip: boolean;
};

export function treasonRemoveSites(g: CatanState, player: number): number[] {
  const q = g.citiesKnights?.pending;
  if (!q || q.kind !== "treason_remove" || q.players[0] !== player) return [];
  const neutral = !!g.two?.knights && q.source === "two_neutral";
  const knights = (g.citiesKnights?.knights || []).filter(
    (n) => n.owner === (neutral ? q.color : player),
  );
  const weakest = Math.min(...knights.map((n) => n.strength));
  return knights
    .filter((n) => !neutral || n.strength === weakest)
    .map((n) => n.vertex);
}

// Never construct a response from another player's view or a stale map selection.
export function progressChoiceAction(
  room: Room,
  selection: ProgressChoiceSelection,
): Record<string, unknown> | null {
  const action = progressChoicePayload(room, selection);
  const x = room.game?.catan?.explorer;
  return action && x ? { ...action, prompt: x.sequence } : action;
}

function progressChoicePayload(
  room: Room,
  selection: ProgressChoiceSelection,
): Record<string, unknown> | null {
  const g = room.game?.catan,
    q = g?.citiesKnights?.pending,
    p = room.you;
  if (
    !g ||
    !q ||
    room.spectating ||
    room.status !== "playing" ||
    room.game?.finished ||
    p < 0 ||
    g.players[p]?.eliminated ||
    q.players[0] !== p ||
    room.game?.phase !== `catan_${q.kind}`
  )
    return null;
  const type = `catan_${q.kind}`;
  if (selection.skip)
    return ["diplomacy", "espionage", "treason_place"].includes(q.kind)
      ? { type, choice: "skip" }
      : null;
  if (["guild_dues", "wedding", "sabotage"].includes(q.kind)) {
    const hand = progressChoiceHand(g, p),
      bundle = selection.bundle;
    if (
      hand.length !== 8 ||
      bundle.length !== 8 ||
      bundle.some((n, i) => !Number.isInteger(n) || n < 0 || n > hand[i]) ||
      sum(bundle) !== progressChoiceDue(g, p)
    )
      return null;
    return { type, [q.kind === "guild_dues" ? "take" : "give"]: bundle };
  }
  if (q.kind === "commercial_harbor") {
    const color = selection.color;
    return color !== null &&
      [5, 6, 7].includes(color) &&
      (g.players[p].resources?.[color] || 0) > 0
      ? { type, color }
      : null;
  }
  if (q.kind === "espionage") {
    const card = selection.card;
    return card !== null &&
      q.progress?.includes(card) &&
      ![9, 23].includes(card)
      ? { type, card }
      : null;
  }
  const map = selection.map;
  if (!map || map.type !== q.kind) return null;
  if (q.kind === "diplomacy")
    return g.diplomacyPlacements?.includes(map.id)
      ? { type, edge: map.id }
      : null;
  if (q.kind === "treason_remove")
    return treasonRemoveSites(g, p).includes(map.id)
      ? { type, vertex: map.id }
      : null;
  if (
    q.kind === "treason_place" &&
    g.treasonPlacements?.includes(map.id) &&
    selection.color !== null &&
    treasonStrengths(g, p).includes(selection.color)
  )
    return { type, vertex: map.id, color: selection.color };
  return null;
}
