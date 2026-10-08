import type { Room, CatanState } from "./types";

// Running games are authoritative: a stale room draft must never change the
// rules shown for a saved game. Waiting rooms use the approved configuration.
export function catanRuleContext(room: Room) {
  const game = room.game?.catan;
  const explorerDraft = [
    "land-ho",
    "spices-for-catan",
    "pirate-lairs",
    "fish-for-catan",
    "explorers-and-pirates",
  ].includes(room.catanScenario || "");
  const sea = game?.seafarers;
  const citySetup = game ? game.citiesKnights : room.catanCitiesKnights;
  const citiesKnights = !!citySetup;
  const harbors = game ? !!game.harbors : !!room.catanHarbors?.enabled;
  const caravans = game
    ? !!game.caravans
    : (room.catanTwoScenario || room.catanScenario) === "caravans";
  const attack = game
    ? !!game.attack
    : room.catanScenario === "barbarian-attack";
  const transport = game
    ? !!game.transport
    : room.catanScenario === "transport";
  const players = game ? game.players.length : room.capacity;
  const options = game ? game.options || {} : room.catanOptions || {};
  let scenario = game
    ? game?.explorer?.board.scenario ||
      sea?.scenario ||
      (sea?.newWorld ? "new_world" : sea?.wonders ? "wonders" : "")
    : (explorerDraft ? room.catanScenario : "") ||
      room.catanSeafarers?.scenario ||
      (room.catanNewWorldMap ? "new_world" : "");
  if (scenario === "islands" && players > 4) scenario = "six_islands";
  const layout =
    citiesKnights && !scenario
      ? citySetup?.layout || "variable"
      : game
        ? game.explorer?.board.layout ||
          sea?.layout ||
          (sea?.newWorld ? "prepared" : sea?.variable ? "variable" : "fixed")
        : (explorerDraft
            ? room.catanScenario === "land-ho"
              ? "fixed"
              : "variable"
            : "") ||
          room.catanSeafarers?.layout ||
          (room.catanNewWorldMap ? "prepared" : "");
  const fixedBase =
    !scenario &&
    !citiesKnights &&
    (game ? game.baseSetup?.layout : room.catanBaseConfiguration?.layout) ===
      "fixed";
  return {
    eventFleet: game
      ? game.eventDeck?.fleetRules || ""
      : room.catanEvents && scenario === "pirate_islands"
        ? "wire-board-events-fleet-v1"
        : "",
    fishingNumberRecipe: game
      ? game.fishing?.map?.numberRecipe || ""
      : players > 4 && room.catanScenario === "fishing"
        ? "wire-board-extended-numbers-v1"
        : "",
    seaNumberRecipe: game
      ? sea?.numberRecipe || ""
      : players > 4 && scenario === "shores"
        ? "wire-board-extended-numbers-v1"
        : "",
    eventClothFallback: game
      ? game.eventDeck?.clothFallback || ""
      : room.catanEvents && scenario === "cloth"
        ? "wire-board-events-cloth-fallback-v1"
        : "",
    eventKnights: game
      ? game.eventDeck?.knights || ""
      : room.catanEvents && citiesKnights
        ? "wire-board-events-knights-v1"
        : "",
    events: game ? game.eventDeck?.catalogue || "" : room.catanEvents || "",
    explorer: game ? !!game.explorer : explorerDraft,
    fishing: game
      ? !!game.fishing
      : room.catanScenario === "fishing" || !!room.catanFishing,
    transport,
    attack,
    caravans,
    rivers: game
      ? !!game.rivers
      : (room.catanTwoScenario || room.catanScenario) === "rivers",
    two: game
      ? !!game.two
      : !!room.catanTwoRules || (transport && players === 2),
    citiesKnights,
    harbors,
    friendlyKnights: game
      ? game.friendlyRobber?.knights || ""
      : room.catanFriendlyRobber?.enabled && citiesKnights
        ? "wire-board-friendly-knights-v1"
        : "",
    friendlyFishingFallback: game
      ? game.friendlyRobber?.fallback ===
        "wire-board-friendly-fishing-fallback-v1"
      : !!room.catanFriendlyRobber?.enabled &&
        (room.catanScenario === "fishing" || !!room.catanFishing),
    friendlySeaFallback: game
      ? game.friendlyRobber?.fallback === "wire-board-friendly-sea-fallback-v1"
        ? game.friendlyRobber.fallback
        : ""
      : room.catanFriendlyRobber?.enabled &&
          room.catanSeafarers &&
          !room.catanFishing
        ? "wire-board-friendly-sea-fallback-v1"
        : "",
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
    fiveSix: game
      ? !!game.paired || !!options.fiveSix
      : !!options.fiveSix || ((explorerDraft || attack) && players > 4),
    helpers: !!options.helpers,
    allHelpers: !!options.allHelpers,
    target: game
      ? catanSavedVictoryTarget(game)
      : transport
        ? 13
        : caravans || attack
          ? 12
          : catanVictoryTarget(scenario, citiesKnights) + (harbors ? 1 : 0),
  };
}

export type CatanRuleContext = ReturnType<typeof catanRuleContext>;

export function catanVictoryTarget(scenario: string, citiesKnights: boolean) {
  const target = (
    {
      "land-ho": 8,
      "spices-for-catan": 15,
      "pirate-lairs": 12,
      "fish-for-catan": 15,
      "explorers-and-pirates": 17,
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
    ? target +
        (citiesKnights
          ? [
              "pirate-lairs",
              "fish-for-catan",
              "spices-for-catan",
              "explorers-and-pirates",
            ].includes(scenario)
            ? 5
            : 2
          : 0)
    : citiesKnights
      ? 13
      : 10;
}

// New views provide the server-derived target; old saves retain the fallback.
export function catanSavedVictoryTarget(g: CatanState) {
  if (g.victoryTarget && g.victoryTarget > 0) return g.victoryTarget;
  if (g.explorer) return g.explorer.board.target;
  if (g.transport) return 13;
  if (g.caravans || g.attack) return 12;
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
