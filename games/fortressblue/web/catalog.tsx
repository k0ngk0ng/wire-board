import { Cover } from "./Cover";
export default {
  id: "fortressblue",
  name: "疯狂坦克",
  category: "sports",
  description:
    "参考《Fortress 2 BLUE》的团队炮战：选择坦克，读风调角，蓄力开炮，在可破坏地形上争夺位置。",
  minPlayers: 2,
  maxPlayers: 8,
  playerCounts: [2, 4, 6, 8],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
