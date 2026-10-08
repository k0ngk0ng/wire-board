import type { Room } from "./types";

export const CATAN_EVENT_CATALOGUE = "wire-board-events-v1";
export function catanEventsSupported(room: Partial<Room>) {
  return (
    room.kind === "catan" &&
    !room.catanCitiesKnights &&
    !room.catanFishing &&
    !room.catanHarbors?.enabled &&
    !room.catanFriendlyRobber?.enabled &&
    !room.catanNewWorldMap &&
    [
      "",
      "rivers",
      "caravans",
      "barbarian-attack",
      "shores",
      "islands",
      "fog",
      "desert",
    ].includes(room.catanSeafarers?.scenario || room.catanScenario || "")
  );
}
