import { Cover } from "./Cover";
export default {
  id: "tabletennis",
  name: "乒乓球",
  category: "sports",
  description:
    "站在球台前，划动球拍对拉、削球和扣杀。支持单打与搭档交替击球的双打。",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 4],
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover,
} as const;
