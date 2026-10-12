import type { Card } from "./types";

// Original 5 × 5 atlas: 247 × 343 pixels per cell. Colored illustrations
// follow black/red/white/blue/green, unlike our game color order.
const colorColumns = [4, 2, 3, 0, 1];
export function orientArtCell(card: Card): number | undefined {
  switch (card.orient) {
    case "copy":
      return 0;
    case "gold":
      return 1;
    case "copy_cascade":
      return 2;
    case "double":
    case "sacrifice":
    case "cascade": {
      const column = colorColumns[card.color];
      if (column === undefined) return undefined;
      const row = { double: 1, sacrifice: 2, cascade: 3 }[card.orient];
      return row * 5 + column;
    }
    default:
      return undefined;
  }
}

export function orientAtlasStyle(assets: string, cell: number | undefined) {
  if (!assets || cell === undefined) return undefined;
  return {
    backgroundImage: `url("${assets}/splendor/expansions/orient-cards-v1.webp")`,
    backgroundSize: "500% 500%",
    backgroundPosition: `${(cell % 5) * 25}% ${Math.floor(cell / 5) * 25}%`,
    backgroundRepeat: "no-repeat",
  };
}

export function orientBackStyle(assets: string, tier: number) {
  return orientAtlasStyle(
    assets,
    tier >= 1 && tier <= 3 ? 19 + tier : undefined,
  );
}
