import { defineGame } from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { palResidentResources } from "./resident-assets";
import { Board } from "./Board";
import type { PalView } from "./view";

export default defineGame<PalView>({
  ...catalog,
  asyncBoard: true,
  streamInputOnly: true,
  assetLoading: "board",
  resources: () => [],
  residentResources: room => palResidentResources(room.game?.data as PalView),
  isInputRelease: (a) => (a.type === "input" || a.type === "local_input" || a.type === "local_release") && a.direction === 4 && !a.keys,
  Board,
  Rules: () => (
    <p>玩家操作全队，按原版剧情探索与战斗。支持保存并退出，使用原账号继续。</p>
  ),
  PlayerSummary: ({ data, seat }) => (
    <>
      {data.party
        .filter((p) => p.controller === seat)
        .map((p) => (
          <span key={p.role} className="pal-player-summary">
            <strong>{p.name}</strong>
            <span>
              体力 {p.hp}/{p.maxHP}
            </span>
            <span>
              真气 {p.mp}/{p.maxMP}
            </span>
          </span>
        ))}
    </>
  ),
});
