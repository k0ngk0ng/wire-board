import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { RailBoard } from "./board";
export default defineGame({
  ...catalog,
  Board: ({ room, act, busy, assets, command }) => (
    <RailBoard room={room as Room} act={act} busy={busy} />
  ),
});
