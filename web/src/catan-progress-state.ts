import type { Room, CatanState } from "./types";
import { cityImprovementReason, cityWallSites } from "./catan-city-state.ts";
export type ProgressSelection = {
  upgrade?: "city" | "harbor";
  numbers?: number[];
  card: number;
  color: number | null;
  target: number | null;
  dice: number[];
  picks: number[];
  skip: boolean;
};
export const newProgressSelection = (card: number): ProgressSelection => ({
  card,
  color: null,
  target: null,
  dice: [1, 1],
  picks: [],
  skip: false,
});
export const progressTrack = (card: number) =>
  card < 10 ? 0 : card < 16 ? 1 : 2;
export function progressMapMode(
  card: number,
  explorer = false,
  attackCity = false,
) {
  if (attackCity && card === 8) return "progress_edge";
  if (attackCity && card === 19) return "progress_tile";
  if (card === 21 && explorer) return "";
  return [3, 12, 21].includes(card)
    ? "progress_tile"
    : [2, 5, 8, 19].includes(card)
      ? "progress_vertex"
      : card === 16
        ? "progress_edge"
        : "";
}
export function progressOpponents(g: CatanState, player: number, card: number) {
  const targets = g.players.flatMap((p, i) =>
    i === player ||
    p.eliminated ||
    (card === 11 && !g.guildDuesTargets?.includes(i)) ||
    (card === 22 &&
      !(g.attack?.city?.knights || g.citiesKnights?.knights)?.some(
        (n) => n.owner === i,
      ))
      ? []
      : [i],
  );
  if (card === 22 && g.two?.knights)
    targets.push(
      ...[-2, -3].filter((owner) =>
        g.citiesKnights?.knights.some((n) => n.owner === owner),
      ),
    );
  return targets;
}
export function progressMapTargets(
  g: CatanState,
  player: number,
  s: ProgressSelection,
) {
  switch (s.card) {
    case 2:
      return cityWallSites(g, player);
    case 3:
      return g.inventionTiles || [];
    case 5:
      if (g.explorer && s.upgrade === "harbor") return g.medicineHarbors || [];
      return (g.players[player]?.resources?.[3] || 0) >= 1 &&
        (g.players[player]?.resources?.[4] || 0) >= 2
        ? g.legal.cities
        : [];
    case 8:
      return [
        ...new Set([
          ...s.picks,
          ...(g.smithingOptions || [])
            .filter((a) => s.picks.every((n, i) => a[i] === n))
            .flatMap((a) => a.slice(s.picks.length, s.picks.length + 1)),
        ]),
      ];
    case 12:
      return g.merchantTiles || [];
    case 16:
      return g.diplomacyRoads || [];
    case 19:
      return g.attack?.city
        ? g.attack.city.choices?.capture || []
        : g.intrigueTargets || [];
    case 21:
      if (g.explorer) return [];
      if (g.taxationTiles !== undefined) return g.taxationTiles;
      return g.citiesKnights?.invasions
        ? g.tiles
            .filter(
              (t) => t.id !== g.robber && t.resource >= 0 && t.resource <= 5,
            )
            .map((t) => t.id)
        : [];
    default:
      return [];
  }
}
export function pickProgressTarget(
  g: CatanState,
  player: number,
  s: ProgressSelection,
  id: number,
): ProgressSelection {
  if (!progressMapTargets(g, player, s).includes(id)) return s;
  if (s.card === 8) {
    const i = s.picks.indexOf(id);
    return {
      ...s,
      skip: false,
      picks: i >= 0 ? s.picks.slice(0, i) : [...s.picks, id],
    };
  }
  if (s.card === 3) {
    const picks = s.picks.includes(id)
      ? s.picks.filter((n) => n !== id)
      : [...s.picks.slice(-1), id];
    return {
      ...s,
      skip: false,
      picks,
      numbers: picks.map((tile) => {
        const previous = s.picks.indexOf(tile);
        return previous >= 0 && s.numbers?.[previous] !== undefined
          ? s.numbers[previous]
          : (progressNumberOptions(g, tile)[0]?.slot ?? 0);
      }),
    };
  }
  return { ...s, skip: false, picks: [id] };
}
export function progressNumberOptions(g: CatanState, tile: number) {
  return (
    g.inventionNumbers?.filter((n) => n.tile === tile) || [
      { tile, slot: 0, number: g.tiles[tile]?.number || 0 },
    ]
  );
}
export function progressGain(g: CatanState, player: number, card: number) {
  const color = card === 4 ? 3 : 4;
  return Math.min(
    g.bank[color],
    2 *
      g.tiles.filter(
        (t) =>
          t.resource === color &&
          t.vertices.some(
            (id) => g.vertices[id].owner === player && g.vertices[id].level > 0,
          ),
      ).length,
  );
}
export function progressPlayAction(
  room: Room,
  s: ProgressSelection,
): Record<string, unknown> | null {
  const g = room.game?.catan,
    p = room.you;
  if (
    !g?.citiesKnights ||
    room.spectating ||
    room.status !== "playing" ||
    room.game?.finished ||
    p < 0 ||
    g.players[p]?.eliminated ||
    room.game?.turn !== p ||
    g.citiesKnights.pending ||
    !g.citiesKnights.players[p].progress?.includes(s.card) ||
    !g.progressPlayable?.includes(s.card) ||
    [9, 23].includes(s.card)
  )
    return null;
  if (room.game.phase !== (s.card === 0 ? "catan_roll" : "catan_turn"))
    return null;
  const a: Record<string, unknown> = {
    type: "catan_progress",
    card: s.card,
    ...(g.explorer ? { prompt: g.explorer.sequence } : {}),
  };
  if (s.skip) return s.card !== 0 ? { ...a, choice: "skip" } : null;
  if (s.card === 0)
    return s.dice.length === 2 &&
      s.dice.every((n) => Number.isInteger(n) && n >= 1 && n <= 6) &&
      !(
        g.transport?.knights &&
        g.players.length <= 4 &&
        [2, 12].includes(s.dice[0] + s.dice[1])
      )
      ? { ...a, tokens: s.dice }
      : null;
  if (s.card === 1)
    return s.color !== null &&
      [0, 1, 2].includes(s.color) &&
      !cityImprovementReason(g, p, s.color, 1)
      ? { ...a, color: s.color }
      : null;
  if ([11, 18, 22].includes(s.card))
    return s.target !== null &&
      progressOpponents(g, p, s.card).includes(s.target)
      ? { ...a, target: s.target }
      : null;
  if ([13, 14, 15].includes(s.card))
    return s.color !== null &&
      Number.isInteger(s.color) &&
      s.color >= (s.card === 15 ? 5 : 0) &&
      s.color < (s.card === 14 ? 5 : 8)
      ? { ...a, color: s.color }
      : null;
  if (s.card === 8)
    return s.picks.length > 0 &&
      (g.smithingOptions || []).some(
        (v) =>
          v.length === s.picks.length && v.every((n, i) => s.picks[i] === n),
      )
      ? { ...a, targets: s.picks }
      : null;
  // E&P Taxation starts a separate pirate placement response; it never asks
  // for a base-game robber tile in the card-play request.
  if (s.card === 21 && g.explorer) return a;
  const mode = progressMapMode(s.card, !!g.explorer, !!g.attack?.city);
  if (mode) {
    const targets = progressMapTargets(g, p, s);
    if (
      s.picks.length !== (s.card === 3 ? 2 : 1) ||
      s.picks.some((n) => !targets.includes(n)) ||
      new Set(s.picks).size !== s.picks.length
    )
      return null;
    if (
      s.card === 3 &&
      s.picks.some(
        (tile, i) =>
          !progressNumberOptions(g, tile).some(
            (n) => n.slot === (s.numbers?.[i] ?? 0),
          ),
      )
    )
      return null;
    return {
      ...a,
      ...(s.card === 3 && s.numbers?.some((slot) => slot !== 0)
        ? { tokens: s.numbers }
        : {}),
      ...(s.card === 5 && g.explorer && s.upgrade === "harbor"
        ? { choice: "harbor" }
        : {}),
      [mode === "progress_tile"
        ? "tile"
        : mode === "progress_vertex"
          ? "vertex"
          : "edge"]: s.picks[0],
      ...(s.card === 3 ? { target: s.picks[1] } : {}),
    };
  }
  return [4, 6, 7, 10, 17, 20, 24].includes(s.card) ? a : null;
}
export function commercialOfferAction(
  room: Room,
  harbor: number | null,
  target: number | null,
  color: number | null,
): Record<string, unknown> | null {
  const g = room.game?.catan,
    p = room.you,
    k = g?.citiesKnights,
    powers = k?.tradePowers;
  if (
    !g ||
    !powers ||
    room.spectating ||
    room.status !== "playing" ||
    room.game?.finished ||
    p < 0 ||
    g.players[p]?.eliminated ||
    room.game?.turn !== p ||
    room.game?.phase !== "catan_turn" ||
    k?.pending ||
    powers.player !== p ||
    harbor === null ||
    target === null ||
    color === null ||
    !powers.harbors[harbor]?.includes(target) ||
    g.players[target]?.eliminated ||
    target === p ||
    ![0, 1, 2, 3, 4].includes(color) ||
    !(g.players[p].resources?.[color] || 0)
  )
    return null;
  return {
    type: "catan_commercial_offer",
    card: harbor,
    target,
    color,
    ...(g.explorer ? { prompt: g.explorer.sequence } : {}),
  };
}
