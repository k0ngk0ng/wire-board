import type { CatanState } from "./types";

// The public map carries every production disc, including the disc moved
// from the lake in Through the Desert. Never infer numbers from artwork.
export function catanProductionNumbers(g: CatanState, tile: number): number[] {
  const t = g.tiles[tile];
  if (!t) return [];
  const lake = g.fishing?.map.lakes?.find((l) => l.tile === tile);
  if (lake) return [...lake.numbers];
  return [
    ...new Set([
      t.number,
      ...(g.rivers?.map.doubleNumberTile === tile ? [2] : []),
      ...(g.rivers?.map.extraNumbers || [])
        .filter((n) => n.tile === tile)
        .map((n) => n.number),
      ...(g.attack?.map?.extraNumbers || [])
        .filter((n) => n.tile === tile)
        .map((n) => n.number),
      ...(g.transport?.map.extraNumbers || [])
        .filter((n) => n.tile === tile)
        .map((n) => n.number),
      ...(g.caravans?.extraNumbers || [])
        .filter((n) => n.tile === tile)
        .map((n) => n.number),
      ...(g.fishing?.map.extraNumbers || [])
        .filter((n) => n.tile === tile)
        .map((n) => n.number),
    ]),
  ].filter((n) => n >= 2 && n <= 12 && n !== 7);
}

export function catanTileProducing(g: CatanState, tile: number, roll: number) {
  return (
    !g.attack?.conquered.includes(tile) &&
    g.robber !== tile &&
    catanProductionNumbers(g, tile).includes(roll)
  );
}
