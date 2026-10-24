import { Cover } from "./Cover";
export default {
  id: "conquest",
  name: "征服时代",
  description:
    "在世界、欧洲、美国或中国地图上经营领土、集结军队，与朋友同时规划、逐轮交锋。",
  category: "board",
  minPlayers: 2,
  maxPlayers: 8,
  supportsBots: true,
  ownsSounds: true,
  Cover,
} as const;
