import { Cover } from "./Cover";
export default {
  id: "monopoly",
  name: "强手棋",
  description:
    "经典地产交易游戏。掷骰买地、公开竞价、交换地产、建房收租，成为最后未破产的玩家。",
  minPlayers: 2,
  maxPlayers: 6,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
