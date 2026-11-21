import { Cover } from "./Cover";

export default {
  id: "hotlinemiami",
  name: "迈阿密热线",
  description: "戴上面具，在霓虹街区展开俯视混战。枪械、近战与武器投掷，一击制胜，迅速重返战场。",
  category: "realtime",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 3, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  streamInputOnly: true,
  Cover,
} as const;
