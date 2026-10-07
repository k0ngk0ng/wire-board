import type { CatanState } from "./types";
export const cityTracks = ["科学", "贸易", "政治"];
export const cityTrackKeys = ["science", "trade", "politics"];
export const cityTrackColors = ["#327742", "#879a33", "#637d99"];
export const cityTrackPowers = [
  "三级：引水渠，非7点且无生产时选一张资源",
  "三级：任意商品可按2:1与银行交易",
  "三级：可以将骑士提升至三级",
];
export const cityActionNames: Record<string, string> = {
  diplomacy: "重放路线",
  treason_remove: "移除骑士",
  treason_place: "放置骑士",
  wall: "建造城墙",
  metropolis: "放置大都会",
  pillage: "降级城市",
  knight_recruit: "招募骑士",
  knight_activate: "激活骑士",
  knight_promote: "升级骑士",
  knight_move: "移动骑士",
  knight_chase: "驱逐强盗",
  knight_chase_pirate: "驱逐海盗",
  knight_retreat: "骑士退让",
};
export const cityActionCosts: Record<string, number[]> = {
  wall: [0, 2, 0, 0, 0],
  knight_recruit: [0, 0, 1, 0, 1],
  knight_activate: [0, 0, 0, 1, 0],
  knight_promote: [0, 0, 1, 0, 1],
};
// A two-point Explorer harbor is not a city: it cannot hold walls or a
// metropolis, unlock improvements, or contribute to the barbarian city count.
export function cityAt(g: CatanState, vertex: number) {
  const v = g.vertices[vertex];
  return !!v && v.level === 2 && !v.harbor && !(g.explorer && !g.citiesKnights);
}
export function cityWallSites(g: CatanState, player: number) {
  const k = g.citiesKnights;
  if (!k || k.walls.filter((v) => g.vertices[v].owner === player).length >= 3)
    return [];
  return g.vertices
    .filter(
      (v) => v.owner === player && cityAt(g, v.id) && !k.walls.includes(v.id),
    )
    .map((v) => v.id);
}
export function cityMetropolisSites(g: CatanState, player: number) {
  const k = g.citiesKnights;
  return k
    ? g.vertices
        .filter(
          (v) =>
            v.owner === player &&
            cityAt(g, v.id) &&
            !k.metropolises.includes(v.id),
        )
        .map((v) => v.id)
    : [];
}
export function cityImprovementReason(
  g: CatanState,
  player: number,
  track: number,
  discount = 0,
) {
  const k = g.citiesKnights,
    level = k?.players[player]?.improvements[track] ?? 5;
  if (!k || level >= 5) return "已达五级";
  if (!g.vertices.some((v) => v.owner === player && cityAt(g, v.id)))
    return "需要至少一座城市";
  const metro = k.metropolises[track],
    owner = metro >= 0 ? g.vertices[metro].owner : -1;
  if (level >= 3 && owner !== player && !cityMetropolisSites(g, player).length)
    return "需要一座没有大都会的城市";
  const cost = Math.max(0, level + 1 - discount);
  if ((g.players[player]?.resources?.[5 + track] ?? 0) < cost)
    return `需要${cost}张${["纸张", "布料", "钱币"][track]}`;
  return "";
}
export function cityDiscardLimit(g: CatanState, player: number) {
  return (
    7 +
    2 *
      (g.citiesKnights?.walls.filter(
        (v) => g.vertices[v].owner === player && cityAt(g, v),
      ).length ?? 0)
  );
}
export function cityDefense(g: CatanState, player?: number) {
  return (
    g.citiesKnights?.knights
      .filter(
        (n) =>
          n.active &&
          !g.players[n.owner].eliminated &&
          (player == null || n.owner === player),
      )
      .reduce((s, n) => s + n.strength, 0) ?? 0
  );
}
