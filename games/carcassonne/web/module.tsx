import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { CarcassonneBoard } from "./carcassonne";
export default defineGame({
  ...catalog,
  Board: ({ room, act, busy, assets, command }) => (
    <CarcassonneBoard
      room={room as Room}
      act={act}
      busy={busy}
      assets={assets}
    />
  ),
});
