import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { CatanBoard } from "./catan";
export default defineGame({
  id: "catan",
  name: "卡坦岛",
  description: "生产、交易和岛屿建设",
  minPlayers: 2,
  maxPlayers: 6,
  supportsBots: true,
  legacy: true,
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
