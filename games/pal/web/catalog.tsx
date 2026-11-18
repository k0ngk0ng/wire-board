import { Cover } from "./Cover";
export default {
  id: "pal",
  name: "仙剑奇侠传",
  description:
    "重温 98 柔情篇，独自探索江湖、指挥全队，随时保存并使用原账号继续。",
  category: "action",
  minPlayers: 1,
  maxPlayers: 1,
  playerCounts: [1],
  requiresFullTable: true,
  supportsBots: true,
  realtime: true,
  ownsSounds: true,
  viewportFit: true,
  viewportAspect: 8 / 5,
  fullscreenToolbarOverlay: true,
  Cover,
} as const;
