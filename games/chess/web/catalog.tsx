import { Cover } from "./Cover";
export default {
  id: "chess",
  name: "国际象棋",
  category: "poker",
  description:
    "王、后、车、象、马与兵各展所长。通过将杀取胜，支持易位、吃过路兵、四种升变与规则和棋，可与朋友或电脑对弈。",
  minPlayers: 2,
  maxPlayers: 2,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
