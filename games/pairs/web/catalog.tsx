import { Cover } from "./Cover";
export default {
  id: "pairs",
  name: "连连看",
  category: "puzzle",
  description:
    "经典图案与闪电连线。1～6 人同图竞速，地图与难度随机，最先清空棋盘的人获胜。",
  minPlayers: 1,
  maxPlayers: 6,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
