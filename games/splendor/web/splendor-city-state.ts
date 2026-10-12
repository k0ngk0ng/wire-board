import type { Game, GemCity, GemPlayer, Noble } from "./types";

// Objectives count physical cards, not permanent discounts. A copied double
// bonus is still one card; an unpaired copy or virtual-gold card is colorless.
export function gemCardCounts(player: GemPlayer) {
  const counts = [0, 0, 0, 0, 0];
  for (const card of player.cards) {
    if (card.color >= 0 && card.color < counts.length) counts[card.color]++;
  }
  return counts;
}

export function gemNobleEligible(player: GemPlayer, noble: Noble) {
  // Match the server's legacy base-save fallback; Orient always needs physical
  // counts so a double bonus cannot qualify for a noble one card too early.
  const counts = player.cards.some((card) => card.orient)
    ? gemCardCounts(player)
    : player.bonus;
  return noble.cost.every((need, color) => counts[color] >= need);
}

export function gemCityProgress(player: GemPlayer, city: GemCity) {
  const counts = gemCardCounts(player);
  // The wildcard must use a color not already required by the printed face.
  const other = counts.reduce(
    (best, count, color) =>
      city.cost[color] === 0 ? Math.max(best, count) : best,
    0,
  );
  return {
    counts,
    other,
    eligible:
      !player.eliminated &&
      player.score >= city.points &&
      city.cost.every((need, color) => counts[color] >= need) &&
      other >= (city.any ?? 0),
  };
}

export function splendorResultDescription(s: NonNullable<Game["splendor"]>) {
  if (s.players.filter((p) => !p.eliminated).length === 1) {
    return "其他玩家已超时离场，最后留在牌桌的玩家获胜。";
  }
  return (
    (s.options?.cities
      ? "满足任一城市的全部条件后完成本轮；仅满足城市条件的玩家参与比较，分数最高者获胜。"
      : "达到 15 分后完成本轮，分数最高者获胜。") +
    "同分时，发展卡更少者获胜；仍相同则共同获胜。超时离场的玩家不参与排名。"
  );
}

export const splendorCityCatalogue = "wire-board-cities-v1";
export const splendorCityGroupingNote =
  "本站城市分组：同名城市的两种条件为一组，七组随机选三组，每组随机一面。14种条件参考BGA，实体正反面配对未逐一核实。";

export function splendorCitySource(
  options: { cities?: boolean } | undefined,
  catalog: string | undefined,
  waiting = false,
) {
  if (!options?.cities) return "";
  if (waiting || catalog === splendorCityCatalogue)
    return splendorCityGroupingNote;
  return "本局沿用开局时保存的城市配置；实体正反面配对未逐一核实。";
}
