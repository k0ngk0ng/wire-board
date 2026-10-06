import type { Room, CatanState } from "./types";

// Running games are authoritative: a stale room draft must never change the
// rules shown for a saved game. Waiting rooms use the approved configuration.
export function catanRuleContext(room: Room) {
  const game = room.game?.catan;
  const sea = game?.seafarers;
  const citySetup = game ? game.citiesKnights : room.catanCitiesKnights;
  const citiesKnights = !!citySetup;
  const harbors = game ? !!game.harbors : !!room.catanHarbors?.enabled;
  const caravans = game
    ? !!game.caravans
    : room.catanTwoScenario === "caravans";
  const players = game ? game.players.length : room.capacity;
  const options = game ? game.options || {} : room.catanOptions || {};
  let scenario = game
    ? sea?.scenario ||
      (sea?.newWorld ? "new_world" : sea?.wonders ? "wonders" : "")
    : room.catanSeafarers?.scenario ||
      (room.catanNewWorldMap ? "new_world" : "");
  if (scenario === "islands" && players > 4) scenario = "six_islands";
  const layout =
    citiesKnights && !scenario
      ? citySetup?.layout || "variable"
      : game
        ? sea?.layout ||
          (sea?.newWorld ? "prepared" : sea?.variable ? "variable" : "fixed")
        : room.catanSeafarers?.layout ||
          (room.catanNewWorldMap ? "prepared" : "");
  const fixedBase =
    !scenario &&
    !citiesKnights &&
    (game ? game.baseSetup?.layout : room.catanBaseConfiguration?.layout) ===
      "fixed";
  return {
    caravans,
    rivers: game ? !!game.rivers : room.catanTwoScenario === "rivers",
    two: game ? !!game.two : !!room.catanTwoRules,
    citiesKnights,
    harbors,
    friendlyRobber: game
      ? !!game.friendlyRobber
      : !!room.catanFriendlyRobber?.enabled,
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
    target: game
      ? catanSavedVictoryTarget(game)
      : caravans
        ? 12
        : catanVictoryTarget(scenario, citiesKnights) + (harbors ? 1 : 0),
  };
}

export type CatanRuleContext = ReturnType<typeof catanRuleContext>;

export function catanVictoryTarget(scenario: string, citiesKnights: boolean) {
  const target = (
    {
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
    } as Record<string, number>
  )[scenario];
  return target !== undefined
    ? target + (citiesKnights ? 2 : 0)
    : citiesKnights
      ? 13
      : 10;
}

// New views provide the server-derived target; old saves retain the fallback.
export function catanSavedVictoryTarget(g: CatanState) {
  if (g.victoryTarget && g.victoryTarget > 0) return g.victoryTarget;
  if (g.caravans) return 12;
  const sea = g.seafarers;
  const scenario =
    sea?.scenario ||
    (sea?.wonders
      ? "wonders"
      : sea?.cloth
        ? "cloth"
        : sea?.pirateIslands
          ? "pirate_islands"
          : sea?.newWorld
            ? "new_world"
            : "");
  return (
    (sea?.victoryPoints || catanVictoryTarget(scenario, !!g.citiesKnights)) +
    (g.harbors ? 1 : 0)
  );
}
