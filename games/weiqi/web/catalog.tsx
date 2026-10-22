import { Cover } from "./Cover";
export default {
  id: "weiqi",
  name: "围棋",
  category: "board",
  description:
    "围地、提子，黑白交替落子。支持九路、十三路与十九路，双方确认死子后按面积计分，可与朋友或电脑对弈。",
  minPlayers: 2,
  maxPlayers: 2,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  Cover: Cover,
} as const;
