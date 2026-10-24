import { Cover } from "./Cover";
export default {
  id: "gomoku",
  name: "五子棋",
  category: "poker",
  description:
    "黑白轮流落子，争取先连成五子。支持无禁手连五与恰好五子两种规则，可与朋友或电脑对弈。",
  minPlayers: 2,
  maxPlayers: 2,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
