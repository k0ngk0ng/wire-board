import {
  classicTerrainFromCatalog,
  type ClassicTerrainCatalog,
  type ClassicTerrainWorld,
} from "./classic-terrain.ts";

type RouteIdentity = {
  id: string;
  geometryHash: string;
  origin?: number[];
  sourceTransform?: number[];
};
type NativeTerrain = Pick<ClassicTerrainCatalog, "version" | "id" | "surfaces"> & {
  origin: number[];
  sourceTransform: number[];
};
const same = (a: unknown, b: unknown) =>
  Array.isArray(a) && Array.isArray(b) && a.length === 3 && b.length === 3 &&
  a.every((value, index) => Number.isFinite(value) && value === b[index]);

/** Vertices already include the original reflection/origin; only units change. */
export function classicTerrainForRoute(
  value: unknown,
  route: RouteIdentity,
  sourceSha256: string,
  track: { geometry?: string },
): ClassicTerrainWorld {
  const raw = value as NativeTerrain | null;
  if (!raw || raw.version !== 1 || raw.id !== route.id ||
      !Array.isArray(raw.surfaces) || !raw.surfaces.length || !same(raw.origin, route.origin) ||
      !same(raw.sourceTransform, route.sourceTransform) ||
      !/^[a-f0-9]{64}$/.test(sourceSha256) || route.geometryHash !== track.geometry)
    throw Error("赛道地表与道路版本不一致");
  const world = classicTerrainFromCatalog({
    version: 1,
    unit: "metres",
    id: raw.id,
    geometry: route.geometryHash,
    sourceSha256,
    surfaces: raw.surfaces,
  }, track);
  if (!world.triangles.length) throw Error("赛道地表缺少有效接触面");
  return world;
}
