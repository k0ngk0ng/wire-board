import { Cover } from "./Cover";

export default {
  id: "pokepath",
  name: "宝可梦：巡径守卫",
  description: "与伙伴沿路布阵，培养、进化并搭配宝可梦与道具，守住 40 条路线的百波挑战。支持单人游玩与多人合作，共享编队与成长。",
  category: "realtime",
  minPlayers: 1,
  maxPlayers: 4,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover,
} as const;
