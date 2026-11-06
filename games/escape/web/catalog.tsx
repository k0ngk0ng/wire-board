import { Cover } from "./Cover";
export default {
  id: "escape",
  name: "密室逃脱",
  description:
    "结伴闯入封锁站与沉没档案馆。分头调查、交换线索、修复机关，在不断变化的险情中一起撤离。每局重组布局与谜题，支持电脑伙伴。",
  category: "puzzle",
  minPlayers: 2,
  maxPlayers: 4,
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  Cover,
} as const;
