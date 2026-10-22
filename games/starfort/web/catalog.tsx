import { Cover } from "./Cover";
export default {
  id: "starfort",
  category: "realtime",
  name: "星垒",
  description:
    "筑塔守住星光。1～4 人探索不断扩展的战场，选择道路、协力成长，在多路敌潮中挑战闯关或无尽守望。",
  minPlayers: 1,
  maxPlayers: 4,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
