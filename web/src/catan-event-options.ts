import type { Room } from "./types";

export const CATAN_EVENT_CATALOGUE = "wire-board-events-v1";
export function catanEventsSupported(room: Partial<Room>) {
  return (
    room.kind === "catan" &&
    (!room.catanFishing ||
      [
        "rivers",
        "caravans",
        "attack-desert",
        "attack-shores",
        "barbarian-attack",
        "transport",
      ].includes(room.catanTwoScenario || room.catanScenario || "") ||
      (room.capacity === 2 && room.catanTwoScenario === "cities-knights") ||
      [
        "land-ho",
        "pirate-lairs",
        "fish-for-catan",
        "spices-for-catan",
        "explorers-and-pirates",
        "shores",
        "islands",
        "fog",
        "desert",
        "tribe",
        "cloth",
        "wonders",
        "new_world",
      ].includes(room.catanSeafarers?.scenario || room.catanScenario || "")) &&
    (!room.catanNewWorldMap ||
      room.catanScenario === "caravans-new-world" ||
      (room.catanSeafarers?.scenario || room.catanScenario) === "new_world") &&
    [
      "",
      "cities-knights",
      "land-ho",
      "pirate-lairs",
      "fish-for-catan",
      "spices-for-catan",
      "explorers-and-pirates",
      "fishing",
      "rivers",
      "caravans",
      "caravans-shores",
      "caravans-desert",
      "caravans-new-world",
      "caravans-islands",
      "caravans-tribe",
      "attack-desert",
      "attack-shores",
      "barbarian-attack",
      "transport",
      "rivers-caravans",
      "rivers-attack",
      "rivers-transport",
      "caravans-attack",
      "caravans-transport",
      "attack-transport",
      "rivers-shores",
      "rivers-fog",
      "rivers-desert",
      "rivers-desert-belt",
      "rivers-tribe",
      "rivers-new-world",
      "shores",
      "islands",
      "fog",
      "desert",
      "tribe",
      "pirate_islands",
      "cloth",
      "wonders",
      "new_world",
    ].includes(room.catanSeafarers?.scenario || room.catanScenario || "")
  );
}
