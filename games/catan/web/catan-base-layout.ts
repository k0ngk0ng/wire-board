import type { Room } from "./types";

export function catanBaseLayoutName(layout: string) {
  return { fixed: "固定新手布局", variable: "随机布局" }[layout] || "";
}

export function supportsExtendedBaseVariants(room: Room) {
  return (
    room.kind === "catan" &&
    room.capacity >= 5 &&
    room.capacity <= 6 &&
    !!room.catanOptions?.fiveSix &&
    !room.catanScenario &&
    !room.catanTwoRules &&
    !room.catanFishing &&
    !room.catanSeafarers &&
    !room.catanNewWorldMap &&
    !room.catanCitiesKnights
  );
}
