import type { Room } from "./types";

export const CATAN_EVENT_CATALOGUE = "wire-board-events-v1";
export function catanEventsSupported(room: Partial<Room>) {
  return (
    room.kind === "catan" &&
    !room.catanFishing &&
    (!room.catanNewWorldMap ||
      (room.catanSeafarers?.scenario || room.catanScenario) === "new_world") &&
    [
      "",
      "cities-knights",
      "rivers",
      "caravans",
      "barbarian-attack",
      "shores",
      "islands",
      "fog",
      "desert",
      "cloth",
      "wonders",
      "new_world",
    ].includes(room.catanSeafarers?.scenario || room.catanScenario || "")
  );
}
