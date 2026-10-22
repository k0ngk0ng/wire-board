import { Cover } from "./Cover";
export default {
  id: "guandan",
  name: "掼蛋",
  description:
    "江苏掼蛋，固定对家、红心配牌，进贡还贡、配合接风，一起打到过 A。",
  category: "poker",
  minPlayers: 4,
  maxPlayers: 4,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
