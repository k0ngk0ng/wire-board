import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { SplendorBoard } from "./board";
export default defineGame({
  ...catalog,
  Board: ({ room, act, busy, assets, command }) => (
    <SplendorBoard room={room as Room} act={act} busy={busy} />
  ),
});
