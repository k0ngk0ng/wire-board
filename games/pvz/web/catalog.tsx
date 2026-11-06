import { Cover } from "./Cover";
export default {
  id: "pvz",
  name: "植物大战僵尸",
  description: "在经典草坪上经营植物防线，或组建僵尸军团突破房屋。支持 1v1、2v2，个人资源独立，双方交换攻守。",
  category: "realtime",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover,
} as const;
