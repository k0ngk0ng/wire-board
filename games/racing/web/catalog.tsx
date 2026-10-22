import { Cover } from "./Cover";
export default {
  id: "racing",
  name: "极品飞车",
  description:
    "从霓虹街区驶向海岸与峡谷，在五条长赛道上追逐对手，掌握弯道与手刹。",
  category: "sports",
  minPlayers: 2,
  maxPlayers: 6,
  playerCounts: [2, 3, 4, 5, 6],
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover: Cover,
} as const;
