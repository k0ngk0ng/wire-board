import { defineGame } from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { OptionsForm } from "./Options";
import { Preparation } from "./Preparation";
import { Rules } from "./Rules";
import { characters, teamGroups } from "./labels";
import type { NetworkView, GameOptions } from "./types";
export default defineGame<NetworkView, GameOptions>({
  ...catalog,
  asyncBoard: true,
  assetLoading: "board",
  preparationOwnsOptions: true,
  viewportFit: true,
  playerLayout: {
    density: "compact",
    groups: (room, data) =>
      teamGroups(data?.options ?? (room.options as GameOptions | undefined)),
  },
  defaultOptions: { arena: 0, teamBattle: false, characters: [3, 9] },
  resources: () => [],
  Board,
  Cover,
  Options: OptionsForm,
  Preparation,
  Rules,
  PlayerSummary: ({ data, seat }) => {
    const player = data.players.find((value) => value.seat === seat);
    return player ? (
      <div className="yuyu-player-summary">
        <span>{characters[player.characterID] ?? "角色"}</span>
        <span className="yuyu-player-values">
          <strong data-yuyu-resource="health">
            体力 {Math.max(0, player.health)}
          </strong>
          <strong data-yuyu-resource="energy">
            灵力 {Math.max(0, player.energy)}
          </strong>
        </span>
      </div>
    ) : null;
  },
});
