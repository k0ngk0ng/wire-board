import { Cover } from "./Cover";
export default {
  id: "bubbleparty",
  name: "泡泡堂",
  description:
    "在小区草坪上放下水泡，连锁爆破、收集道具，救出被困队友，击破对手水泡。",
  category: "realtime",
  minPlayers: 2,
  maxPlayers: 8,
  playerCounts: [2, 4, 6, 8],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover: Cover,
} as const;
