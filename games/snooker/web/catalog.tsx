import { Cover } from "./Cover";
export default {
  id: "snooker",
  name: "斯诺克",
  description:
    "纯 2D 俯视球桌。控制力度与旋转，进球走位、做斯诺克；支持单打和双打。",
  category: "sports",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
