import { Cover } from "./Cover";
export default {
  id: "basketball",
  name: "热血篮球",
  description: "街头双人篮球，争抢、传球与扣篮。支持单人、双人合作和双人对战。",
  category: "sports",
  minPlayers: 1,
  maxPlayers: 2,
  playerCounts: [1, 2],
  requiresFullTable: true,
  supportsBots: true,
  realtime: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
