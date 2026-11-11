import { Cover } from "./Cover";
export default {
  id: "racing",
  name: "极品飞车",
  description:
    "驾驶五款经典跑车，穿越法国乡间、海岸、古堡与山林，体验起伏公路、昼夜赛道和各异的操控。",
  category: "sports",
  minPlayers: 2,
  maxPlayers: 6,
  playerCounts: [2, 3, 4, 5, 6],
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover: Cover,
} as const;
