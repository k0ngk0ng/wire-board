import type { Room } from "./types";
type Map = NonNullable<Room["catanRiversWorldMap"]>;

export function swapRiverWorld(
  map: Map,
  a: number,
  b: number,
  numbersOnly: boolean,
): Map {
  if (a === b || !map.hexes[a] || !map.hexes[b]) return map;
  const river = new Set(map.channels.flatMap((c) => c.tiles));
  if (!numbersOnly && (river.has(a) || river.has(b))) return map;
  if (numbersOnly && (!map.hexes[a].number || !map.hexes[b].number)) return map;
  const hexes = map.hexes.map((h) => ({ ...h }));
  if (numbersOnly)
    [hexes[a].number, hexes[b].number] = [hexes[b].number, hexes[a].number];
  else [hexes[a], hexes[b]] = [hexes[b], hexes[a]];
  return { ...map, hexes };
}
