import { Cover } from "./Cover";

export default {
  id: "contra",
  name: "魂斗罗",
  description:
    "经典横版射击，一人挑战或双人合作闯过八关。真人挑战通关共同加分，失败共同扣分。",
  category: "action",
  ratingPolicy: "coop_campaign_v1",
  minPlayers: 1,
  maxPlayers: 2,
  playerCounts: [1, 2],
  requiresFullTable: true,
  supportsBots: false,
  mobilePlayNotice: true,
  realtime: true,
  ownsSounds: true,
  Cover,
} as const;
