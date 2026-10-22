import { Cover } from "./Cover";
export default {
  id: "junqi",
  name: "军棋",
  category: "board",
  description:
    "布阵藏锋，沿铁路调兵夺旗。支持两人陆战棋和四国对家搭档，暗子只向本人展示，可与朋友或电脑对战。",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
