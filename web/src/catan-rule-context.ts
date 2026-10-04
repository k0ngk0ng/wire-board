import type { Room } from "./types";

// Running games are authoritative: a stale room draft must never change the
// rules shown for a saved game. Waiting rooms use the approved configuration.
export function catanRuleContext(room: Room) {
  const game = room.game?.catan;
  const sea = game?.seafarers;
  const players = game ? game.players.length : room.capacity;
  const options = game ? game.options || {} : room.catanOptions || {};
  let scenario = game
    ? sea?.scenario ||
      (sea?.newWorld ? "new_world" : sea?.wonders ? "wonders" : "")
    : room.catanSeafarers?.scenario ||
      (room.catanNewWorldMap ? "new_world" : "");
  if (scenario === "islands" && players > 4) scenario = "six_islands";
  const layout = game
    ? sea?.layout ||
      (sea?.newWorld ? "prepared" : sea?.variable ? "variable" : "fixed")
    : room.catanSeafarers?.layout || (room.catanNewWorldMap ? "prepared" : "");
  const fixedBase =
    !scenario &&
    (game ? game.baseSetup?.layout : room.catanBaseConfiguration?.layout) ===
      "fixed";
  return {
    scenario,
    layout,
    players,
    waiting: !game,
    fixedBase,
    neutral: game
      ? game.baseSetup?.neutralColor != null && game.baseSetup.neutralColor >= 0
      : fixedBase,
    fiveSix: game ? !!game.paired || !!options.fiveSix : !!options.fiveSix,
    helpers: !!options.helpers,
    allHelpers: !!options.allHelpers,
    target:
      sea?.victoryPoints ||
      ({
        shores: 14,
        islands: 13,
        six_islands: 13,
        fog: 12,
        desert: 14,
        tribe: 13,
        cloth: 14,
        pirate_islands: 10,
        wonders: 10,
        new_world: 12,
      }[scenario] ??
        10),
  };
}

export type CatanRuleContext = ReturnType<typeof catanRuleContext>;
