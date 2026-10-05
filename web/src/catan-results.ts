import type { CatanState } from "./types";

// Describe the rules of the saved game, never a room's mutable next-game draft.
export function catanResultDescription(g: CatanState) {
  if (g.players.filter((p) => !p.eliminated).length === 1)
    return "其他玩家已离场，最后留在牌桌的玩家获胜。";
  const sea = g.seafarers;
  if (sea?.wonders || sea?.scenario === "wonders")
    return `${g.citiesKnights ? "城市与骑士＋" : ""}卡坦奇迹：建成4级奇迹，或达到${sea.victoryPoints || 10}分且奇迹等级独自领先，即可在自己的行动阶段获胜。`;
  if (sea?.cloth || sea?.scenario === "cloth")
    return `${g.citiesKnights ? "城市与骑士＋" : ""}卡坦布匹：在自己的行动阶段达到${sea.victoryPoints || (g.citiesKnights ? 16 : 14)}分获胜；回合结束时至少五座村落的布匹耗尽也会结算，比较总分，同分比较布匹数量。`;
  if (g.citiesKnights)
    return `${g.seafarers ? "航海家＋" : ""}城市与骑士：在自己的行动阶段达到${g.seafarers?.victoryPoints || 13}分获胜。总分包含建筑、大都会、最长路线、防御者、公开进步牌和商人${g.seafarers ? "，以及本剧本的额外得分" : ""}。`;
  if (sea?.pirateIslands || sea?.scenario === "pirate_islands")
    return "海盗群岛：达到10分且夺回自己的要塞，才能在自己的行动阶段获胜。";
  if (sea)
    return `航海家：在自己的行动阶段达到${sea.victoryPoints || 10}分获胜。总分包含建筑、路线奖励、胜利点卡与本剧本的额外得分。`;
  return "在自己的行动阶段达到10分获胜。总分包含建筑、最长道路、最大骑士军队与胜利点卡。";
}
