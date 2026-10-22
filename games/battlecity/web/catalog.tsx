import { Cover } from "./Cover";
export default {
  id: "battlecity",
  name: "坦克大战",
  description:
    "1v1／2v2／3v3 双基地坦克对战：选择坦克、炸开砖墙、争夺随机道具，摧毁敌方大本营。",
  category: "realtime",
  minPlayers: 2,
  maxPlayers: 6,
  playerCounts: [2, 4, 6],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover: Cover,
} as const;
