import { Cover } from "./Cover";
import { mergeNetworkStream } from "./network-stream";
export default {
  id: "yuyuhakusho",
  name: "魔强统一战",
  description: "浦饭幽助与伙伴的灵界格斗，支持二至四人混战、二对二和一对三。",
  category: "realtime",
  minPlayers: 2,
  maxPlayers: 4,
  playerCounts: [2, 3, 4],
  requiresFullTable: true,
  supportsBots: true,
  realtime: true,
  mergeStreamData: mergeNetworkStream,
  ownsSounds: true,
  Cover,
} as const;
