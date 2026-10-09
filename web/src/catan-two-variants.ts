import { supportsTwoCatanSeafarers } from "./catan-two-seafarers.ts";
import {
  supportsTwoCatanHelpers,
  supportsCaravanSeaHelpers,
} from "./catan-two-helpers.ts";
import type { Room } from "./types";

export const supportsTwoCatanVariants = (scenario = "") =>
  ["", "fishing", "cities-knights"].includes(scenario) ||
  supportsTwoCatanSeafarers(scenario) ||
  supportsCaravanSeaHelpers(scenario);

export const twoCatanVariantsAvailable = (room: Room) =>
  room.kind === "catan" &&
  room.capacity === 2 &&
  room.catanTwoRules === "catan-for-two-2025" &&
  !room.catanScenario &&
  supportsTwoCatanVariants(room.catanTwoScenario) &&
  (!room.catanOptions?.helpers ||
    supportsTwoCatanHelpers(room.catanTwoScenario)) &&
  (!room.catanOptions?.allHelpers || !!room.catanOptions?.helpers) &&
  !room.catanOptions?.fiveSix;

export const catanTwoVariantsNote =
  "本站双人组合规则：中立势力不受友善强盗保护，不参与港口霸主奖励；中立建筑仍占据交点并阻断路线，最长路线规则不变。";
