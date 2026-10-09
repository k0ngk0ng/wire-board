import { supportsCatanTradersHelpers } from "./catan-traders-helpers.ts";
export const supportsCaravanSeaHelpers = (s = "") =>
  [
    "caravans-shores",
    "caravans-islands",
    "caravans-desert",
    "caravans-tribe",
    "caravans-new-world",
  ].includes(s);
import { supportsTwoCatanSeafarers } from "./catan-two-seafarers.ts";
export const supportsTwoCatanHelpers = (scenario = "") =>
  ["", "fishing", "cities-knights"].includes(scenario) ||
  supportsTwoCatanSeafarers(scenario) ||
  supportsCaravanSeaHelpers(scenario) ||
  supportsCatanTradersHelpers(scenario);

export const catanTwoHelpersNote =
  "本站双人助手规则：中立势力不持有助手或手牌；两次生产共用一回合的助手机会。助手建造先翻面或交换，再完成中立建设；移动旧路不增加中立道路。助手强制交易只针对唯一真人对手一次，与贸易筹码分别结算。";
