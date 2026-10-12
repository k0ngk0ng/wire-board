import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { SanguoshaBoard } from "./sanguosha";
export default defineGame({
  id: "sanguosha",
  name: "三国杀",
  description: "身份、势力与卡牌对抗",
  minPlayers: 4,
  maxPlayers: 8,
  supportsBots: true,
  legacy: true,
  Board: ({ room, act, busy, assets, command }) => (
    <SanguoshaBoard room={room as Room} act={act} busy={busy} assets={assets} />
  ),
});
