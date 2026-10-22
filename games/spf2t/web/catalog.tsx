import { Cover } from "./Cover";
export default {
  id: "spf2t",
  name: "街霸方块",
  description:
    "双人对战消消乐：同色宝石连成方块，用碎裂宝石引爆连锁，把反击宝石送进对手场地。",
  category: "puzzle",
  minPlayers: 2,
  maxPlayers: 2,
  playerCounts: [2],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover: Cover,
} as const;
