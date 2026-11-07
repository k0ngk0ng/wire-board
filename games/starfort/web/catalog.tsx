import { Cover } from "./Cover";
export default {
  id: "starfort",
  category: "realtime",
  name: "星垒",
  description:
    "1～4 人合作筑塔守护星核，全队选择扩展道路、完成任务，搭配防御塔与被动技能，挑战闯关或无尽守望。",
  minPlayers: 1,
  maxPlayers: 4,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
