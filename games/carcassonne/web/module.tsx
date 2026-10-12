import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { CarcassonneBoard } from "./carcassonne";
export default defineGame({
  id: "carcassonne",
  name: "卡卡颂",
  description: "拼放地块，建设城市与道路",
  minPlayers: 2,
  maxPlayers: 5,
  supportsBots: true,
  legacy: true,
  Board: ({ room, act, busy, assets, command }) => (
    <CarcassonneBoard
      room={room as Room}
      act={act}
      busy={busy}
      assets={assets}
    />
  ),
});
