import { Cover } from "./Cover";
export default {
  id: "tetris",
  name: "俄罗斯方块",
  description: "经典落块对战：旋转与消行，向对手送去垃圾行，或抢先完成 40 行。",
  category: "puzzle",
  minPlayers: 1,
  maxPlayers: 4,
  playerCounts: [1, 2, 3, 4],
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover: Cover,
} as const;
