import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import type { Room } from "./types";
import { DotaBoard } from "./dota";
export default defineGame({
  ...catalog,
  Board: ({ room, act, busy, assets, command }) => (
    <DotaBoard room={room as Room} act={act} busy={busy} assets={assets} />
  ),
});
