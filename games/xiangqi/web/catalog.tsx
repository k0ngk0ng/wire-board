import { Cover } from "./Cover";
export default {
  id: "xiangqi",
  name: "中国象棋",
  category: "board",
  description:
    "楚河汉界，两军对弈。车马炮协同将杀，支持长将、长捉与循环提示，可与朋友或电脑对弈。",
  minPlayers: 2,
  maxPlayers: 2,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
