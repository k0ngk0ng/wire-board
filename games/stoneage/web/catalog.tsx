import { Cover } from "./Cover";
export default {
  id: "stoneage",
  name: "石器竞技场",
  description:
    "共同随机宠物、骑宠、技能、装备与补给，公平选配，展开 1v1 至 5v5 石器回合对战。",
  category: "board",
  minPlayers: 2,
  maxPlayers: 10,
  playerCounts: [2, 4, 6, 8, 10],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  playerLayout: { density: "compact", flow: "horizontal" },
  Cover,
} as const;
