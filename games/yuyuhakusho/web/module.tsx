import { defineGame } from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { OptionsForm } from "./Options";
import { Preparation } from "./Preparation";
import { characters } from "./labels";
import type { NetworkView, GameOptions } from "./types";
export default defineGame<NetworkView, GameOptions>({
  ...catalog,
  asyncBoard: true,
  defaultOptions: { arena: 0, teamBattle: false, characters: [3, 9] },
  // The Board verifies the runtime and exact selected-scene JSON bytes itself.
  // An empty public-media list prevents the host preloading all eight arenas.
  resources: () => [],
  Board,
  Cover,
  Options: OptionsForm,
  Preparation,
  PlayerSummary: ({ data, seat }) => {
    const player = data.players.find((value) => value.seat === seat);
    return player ? (
      <div className="yuyu-player-summary">
        <span>{characters[player.characterID] ?? "角色"}</span>
        <strong>体力 {Math.max(0, player.health)}</strong>
      </div>
    ) : null;
  },
});
