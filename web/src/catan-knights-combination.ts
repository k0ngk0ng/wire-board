import { isPublicCatanRiversSea } from "./catan-rivers-sea-options.ts";
import { isPublicCatanCaravanSea } from "./catan-two-helpers.ts";

// Kept free of JSX so the option predicates stay unit-testable.
export const isCatanTransportSea = (scenario = "") =>
  ["transport-shores", "transport-desert"].includes(scenario);

export const supportsPublicExplorerKnights = (scenario?: string) =>
  [
    "land-ho",
    "pirate-lairs",
    "fish-for-catan",
    "spices-for-catan",
    "explorers-and-pirates",
  ].includes(scenario || "");

export const supportsPublicCatanKnightsCombination = (scenario?: string) =>
  supportsPublicExplorerKnights(scenario) ||
  scenario === "transport" ||
  isCatanTransportSea(scenario) ||
  isPublicCatanRiversSea(scenario) ||
  isPublicCatanCaravanSea(scenario) ||
  [
    "rivers-caravans",
    "rivers-attack",
    "rivers-transport",
    "caravans-attack",
    "caravans-transport",
    "attack-transport",
  ].includes(scenario || "") ||
  scenario === "barbarian-attack" ||
  [
    "rivers",
    "caravans",
    "fishing",
    "shores",
    "islands",
    "fog",
    "desert",
    "new_world",
    "wonders",
    "cloth",
    "tribe",
    "pirate_islands",
  ].includes(scenario || "");
