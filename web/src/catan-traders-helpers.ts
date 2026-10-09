export const CATAN_TRADERS_HELPERS_RULES = "wire-board-traders-helpers-v1";
export const supportsCatanTradersHelpers = (s = "") =>
  [
    "rivers",
    "caravans",
    "barbarian-attack",
    "transport",
    "rivers-caravans",
    "rivers-attack",
    "rivers-transport",
    "caravans-attack",
    "caravans-transport",
    "attack-transport",
    "rivers-shores",
    "rivers-fog",
    "rivers-desert",
    "rivers-desert-belt",
    "rivers-tribe",
    "rivers-new-world",
    "caravans-shores",
    "caravans-islands",
    "caravans-desert",
    "caravans-tribe",
    "caravans-new-world",
    "attack-shores",
    "attack-desert",
    "attack-tribe",
    "attack-wonders",
    "attack-pirates",
    "transport-shores",
    "transport-desert",
  ].includes(s);
export const catanTradersHelpersNote =
  "本站助手组合：保留交易、筑路和生产补偿；蛮族发展牌先私选、翻面或交换，再公开执行。实体骑士可参与建设；无强盗时迪古尔领取金币、卡娅领取普通资源。详细能力以牌桌内的助手说明为准。";
