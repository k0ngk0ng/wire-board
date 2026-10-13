import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { DotaBoard } from "./dota";
export default defineGame({
  id: "dota",
  category: "realtime",
  name: "兵线争锋",
  description: "分队选将，同时规划行动",
  minPlayers: 2,
  maxPlayers: 6,
  playerCounts: [2, 4, 6],
  supportsBots: true,
  legacy: true,
  Board: ({ room, act, busy, assets, command }) => (
    <DotaBoard room={room as Room} act={act} busy={busy} assets={assets} />
  ),
});
