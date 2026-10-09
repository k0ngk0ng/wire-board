export const catanRiversSeaScenarios = [
  {
    id: "rivers-shores",
    name: "河流＋新海岸",
    description:
      "沿河赚金币、修桥并探索外岛，14 分获胜。二至六人；双人与五六人采用本站配方，可加事件牌。",
  },
  {
    id: "rivers-fog",
    name: "河流＋迷雾群岛",
    description:
      "沿河建设并探索迷雾，金矿选择资源，12 分获胜。二至六人；双人与五六人采用本站配方，可加事件牌。",
  },
  {
    id: "rivers-desert",
    name: "河流＋穿越沙漠 · 河流穿越",
    description:
      "河流穿过原沙漠带，探索外岛得分，14 分获胜。二至六人，可加事件牌；五六人使用本站配方。",
  },
  {
    id: "rivers-desert-belt",
    name: "河流＋穿越沙漠 · 保留沙漠",
    description:
      "保留沙漠带，跨越沙漠或探索外岛得分，14 分获胜。二至六人，可加事件牌；五六人使用本站配方。",
  },
  {
    id: "rivers-tribe",
    name: "河流＋遗忘部落",
    description:
      "沿河建设并航行领取部落奖励，13 分获胜。二至四人，可加事件牌。",
  },
  {
    id: "rivers-new-world",
    name: "河流＋新世界",
    description:
      "随机河流地图，先轮流放港口再建设，12 分获胜。二至六人，可加事件牌；当前入口采用默认随机地图。",
  },
];
export const isPublicCatanRiversSea = (id?: string) =>
  catanRiversSeaScenarios.some((item) => item.id === id);
export const catanRiversSeaAvailable = (id: string, players: number) =>
  isPublicCatanRiversSea(id) &&
  players >= 2 &&
  players <= 6 &&
  (players <= 4 ||
    [
      "rivers-shores",
      "rivers-fog",
      "rivers-desert",
      "rivers-desert-belt",
      "rivers-new-world",
    ].includes(id));
