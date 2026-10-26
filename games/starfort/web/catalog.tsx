import { Cover } from "./Cover";
export default {
  id: "starfort",
  category: "realtime",
  name: "星垒",
  description:
    "筑塔守住中央星核。1～4 人各守一路，搭配防御塔与被动技能、协力成长，挑战闯关或无尽守望。",
  minPlayers: 1,
  maxPlayers: 4,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
