export const twoCatanSeaScenarios = [
  {
    id: "shores",
    name: "双人航海家 · 驶向新海岸",
    description: "从主岛驶向外岛，首次定居各个新岛额外得 2 分。",
  },
  {
    id: "islands",
    name: "双人航海家 · 四岛",
    description: "起始定居岛是各自的家乡，在其他岛首次定居额外得 2 分。",
  },
  {
    id: "fog",
    name: "双人航海家 · 迷雾群岛",
    description: "沿路线翻开未知地形，发现资源与金矿。",
  },
  {
    id: "desert",
    name: "双人航海家 · 穿越沙漠",
    description: "越过沙漠或航向外岛，在各个新区域首次定居额外得 2 分。",
  },
  {
    id: "tribe",
    name: "双人航海家 · 遗忘的部落",
    description: "驾船领取部落胜利点、发展卡和港口。",
  },
  {
    id: "cloth",
    name: "双人航海家 · 卡坦布匹",
    description:
      "三组起始村庄；每两枚布匹得 1 分，无最长路线奖励。回合结束时五座村落耗尽也会结算。",
  },
  {
    id: "wonders",
    name: "双人航海家 · 卡坦奇迹",
    description:
      "建成四级奇迹即可获胜；达到目标分数且奇迹等级独自领先也可获胜。",
  },
  {
    id: "new_world",
    name: "双人航海家 · 新世界",
    description:
      "等待房间共同确认地图，轮流放港口；首次定居非家乡岛额外得 1 分。",
  },
];
export const supportsTwoCatanSeafarers = (scenario = "") =>
  twoCatanSeaScenarios.some((s) => s.id === scenario);
export const catanTwoSeafarersNote =
  "本站双人航海家规则：使用四人地图，中立势力各从相隔较远的合法海岸村庄开始。造船后补造中立船，无合法船位才改造中立道路；中立船不移动。中立探索翻开迷雾但不领奖，不占部落奖励边、不与布匹村落贸易。无沙漠时贸易筹码可将强盗退至场外，不影响海盗。每回合完整生产两次。";
