// Keep terrain, interactive edges and labels separate: art texture must not
// wash out the gameplay silhouettes or change semantic text/health colors.
export const battlefieldPalette = {
  terrain: {
    forest: "#87bc51",
    canyon: "#d7a063",
    river: "#78b763",
    snow: "#d8ecec",
    marsh: "#5d9d79",
    ruins: "#a8ab7a",
  },
  terrainEdge: "#29412d",
  textureOpacity: 0.3,
  road: {
    edge: "#513820",
    fill: "#bb874c",
    center: "#cc9b5b",
    inactive: "#a59877",
    inactiveCenter: "#b5aa8c",
  },
  water: { fill: "#369ac6", edge: "#1f4358", ripple: "#c3eff4" },
  highground: { edge: "#665035", top: "#b3bf72" },
  pad: { fill: "#f0e5ad", edge: "#40542d", selected: "#fff1a2" },
  unitOutline: "#243b2d",
  tower: {
    arrow: "#ba8b35",
    cannon: "#a45c36",
    frost: "#5ab8d3",
    arcane: "#a37bcc",
  },
  health: { track: "#203b2b", healthy: "#bfe768", critical: "#ffb482" },
  label: { ink: "#273b2d", danger: "#793d31", halo: "#fff9e8" },
} as const;
