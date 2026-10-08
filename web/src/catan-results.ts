import { explorerScenarioLabel } from "./catan-explorer-state.ts";
import { catanSavedVictoryTarget } from "./catan-rule-context.ts";
import type { CatanState } from "./types";

// Describe the rules of the saved game, never a room's mutable next-game draft.
export function catanResultDescription(g: CatanState) {
  if (g.players.filter((p) => !p.eliminated).length === 1)
    return "其他玩家已离场，最后留在牌桌的玩家获胜。";
  const sea = g.seafarers;
  const target = catanSavedVictoryTarget(g);
  const harbors = g.harbors ? "港口霸主＋" : "";
  if (g.explorer)
    return `探索者与海盗·${explorerScenarioLabel(g)}：在自己的回合达到${target}分获胜。村庄1分、港口2分${g.explorer.lairs ? "，另计巢穴任务进度和领先奖励" : ""}${g.explorer.spice ? "、香料任务进度和领先奖励" : ""}${g.explorer.fish ? "、鱼群任务进度和领先奖励" : ""}，不授予最长道路或最大军队奖励。`;
  if (g.transport)
    return `${g.two ? "双人卡坦＋" : ""}运输任务：在自己的回合达到${target}分立即获胜。总分包含建筑、胜利点卡、最大骑士军队、已交付货物（每件1分）与马车满级奖励（1分）。本剧本不授予最长道路。`;
  if (g.two && g.citiesKnights)
    return `双人城市与骑士${g.caravans ? "＋商队" : ""}：在自己的回合达到${target}分获胜。计入建筑、大都会、最长路线、防御者、公开进步牌与商人${g.caravans ? "，以及相邻至少两辆马车的建筑奖励（每座＋1）" : ""}。中立势力可取得最长路线，中立骑士不参与防御。`;
  if (g.two)
    return `双人卡坦${g.rivers ? "＋河流" : g.caravans ? "＋商队" : ""}：在自己的回合达到${target}分获胜。两家中立势力也可取得最长路线；总分包含建筑、当前持有的路线与军队奖励，以及胜利点卡${g.rivers ? "和当前最富（＋1）／最贫（−2）的财富分数，中立势力不参与财富比较" : g.caravans ? "和相邻至少两辆马车的建筑奖励（每座＋1）" : ""}。`;
  if (sea?.wonders || sea?.scenario === "wonders")
    return `${harbors}${g.citiesKnights ? "城市与骑士＋" : ""}${g.fishing ? "捕鱼＋" : ""}卡坦奇迹：建成4级奇迹，或达到${target}分且奇迹等级独自领先，即可在自己的行动阶段获胜。${g.fishing ? "持旧靴子时分数门槛增加1分，建成4级仍直接获胜。" : ""}`;
  if (sea?.cloth || sea?.scenario === "cloth")
    return `${harbors}${g.citiesKnights ? "城市与骑士＋" : ""}卡坦布匹：在自己的行动阶段达到${target}分获胜；回合结束时至少五座村落的布匹耗尽也会结算，比较总分，同分比较布匹数量。`;
  if (g.citiesKnights)
    return `${harbors}${g.caravans ? "商队＋" : ""}${g.seafarers ? "航海家＋" : ""}城市与骑士：在自己的行动阶段达到${target}分获胜。总分包含建筑、大都会、最长路线、防御者、公开进步牌和商人${g.caravans ? "，以及相邻至少两辆马车的建筑奖励（每座＋1）" : g.seafarers ? "，以及本剧本的额外得分" : ""}。`;
  if (sea?.pirateIslands || sea?.scenario === "pirate_islands")
    return `${harbors}海盗群岛：达到${target}分且夺回自己的要塞，才能在自己的行动阶段获胜。`;
  if (sea)
    return `${harbors}航海家：在自己的行动阶段达到${target}分获胜。总分包含建筑、路线奖励、胜利点卡与本剧本的额外得分。`;
  return `${g.harbors ? "港口霸主：" : ""}在自己的行动阶段达到${target}分获胜。总分包含建筑、最长道路、最大骑士军队与胜利点卡${g.harbors ? "，以及港口霸主奖励" : ""}。`;
}
