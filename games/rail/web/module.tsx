import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { RailBoard } from "./board";
export default defineGame({
  id: "rail",
  name: "铁路环游",
  description: "收集列车牌，连接目的地",
  minPlayers: 2,
  maxPlayers: 5,
  supportsBots: true,
  legacy: true,
  Board: ({ room, act, busy, assets, command }) => (
    <RailBoard room={room as Room} act={act} busy={busy} />
  ),
});
