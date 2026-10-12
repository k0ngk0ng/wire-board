import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { SplendorBoard } from "./board";
export default defineGame({
  id: "splendor",
  name: "璀璨宝石",
  description: "宝石收集与发展卡购买",
  minPlayers: 2,
  maxPlayers: 4,
  supportsBots: true,
  legacy: true,
  Board: ({ room, act, busy, assets, command }) => (
    <SplendorBoard room={room as Room} act={act} busy={busy} />
  ),
});
