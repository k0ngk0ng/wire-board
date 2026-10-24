import { Cover } from "./Cover";
export default {
  id: "doudizhu",
  name: "斗地主",
  description:
    "上海斗地主，四人欢乐三打一。标牌叫档、头撩摊打、报到，地主对抗三位农民。",
  category: "poker",
  tableLayout: "poker",
  minPlayers: 4,
  maxPlayers: 4,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
