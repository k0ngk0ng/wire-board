import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { CatanBoard } from "./catan";
export default defineGame({
  ...catalog,
  Board: ({ room, act, busy, assets, command }) => (
    <CatanBoard
      room={room as Room}
      act={act}
      busy={busy}
      assets={assets}
      resetOpening={() => command("catan_explorer_reset")}
    />
  ),
});
