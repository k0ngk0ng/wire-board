import { Cover } from "./Cover";
export default {
  id: "shengji",
  name: "升级",
  description:
    "上海升级，单副四十分或双副八十分，亮主扣底、搭档抓分，携手打到过 A。",
  category: "poker",
  tableLayout: "poker",
  minPlayers: 4,
  maxPlayers: 4,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
