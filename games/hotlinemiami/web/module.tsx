import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import catalog from "./catalog";
import { Options, Preparation } from "./Preparation";
import { characterNames, isMeleeWeapon, playerColors, weaponNames, type GameOptions, type View } from "./types";
import { Rules } from "./Rules";
import { resizeOptions } from "./options";

export default defineGame<View, GameOptions>({
  ...catalog,
  assetLoading: "board",
  asyncBoard: true,
  preparationOwnsOptions: true,
  resizeOptions,
  fullscreenToolbarOverlay: true,
  playerLayout: { density: "compact" },
  defaultOptions: {
    map: "no-talk",
    roundSeconds: 180,
    characters: ["jacket", "biker"],
    weapons: ["pistol", "pistol"],
  },
  Board,
  Options,
  Preparation,
  isInputRelease: action => action.type === "input" && action.moveX === 0 && action.moveY === 0 && action.fire === false && action.throw === false && action.pickup === false && action.finish === false && action.interact === false,
  Rules,
  PlayerSummary: ({ room, data, seat }) => {
    const player = data.players.find(candidate => candidate.seat === seat);
    if (!player) return null;
    const melee = isMeleeWeapon(player.weapon);
    return <div className="hm-player-summary" data-hotlinemiami-summary={seat} style={{ borderColor: playerColors[seat % 4] }}>
      <strong>{player.alive ? player.downUntil > data.frame ? "倒地" : "存活" : "等待复活"}{seat === room.you ? " · 你" : ""}</strong>
      <span>{characterNames[player.character]} · {player.score} 分</span>
      <span>{weaponNames[player.weapon] ?? player.weapon}{melee ? "" : ` · ${player.ammo} 发`}</span>
    </div>;
  },
  ResultDetails: ({ room, data }) => <div className="hm-results">
    {[...data.players].sort((a, b) => b.score - a.score || b.kills - a.kills || a.seat - b.seat).map(player => <div key={player.seat}>
      <strong>{room.seats[player.seat]?.name ?? "玩家"}</strong><span>{player.score} 分</span><span>击杀 {player.kills} · 死亡 {player.deaths}</span>
    </div>)}
  </div>,
});
