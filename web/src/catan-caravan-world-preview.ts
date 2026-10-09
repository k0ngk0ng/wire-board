import type { CatanNewWorldMap } from "./types";

// Match server map geometry, including rounded shared vertices and tie ordering.
export function caravanWorldPreview(map: CatanNewWorldMap): number[] {
  const rows =
    map.hexes.length === 63 ? [8, 9, 10, 9, 10, 9, 8] : [5, 6, 7, 6, 7, 6, 5];
  if (map.hexes.length !== 42 && map.hexes.length !== 63) return [];
  const starts = [0, -1, -2, -2, -3, -3, -3];
  const raw = rows.flatMap((n, r) =>
    Array.from({ length: n }, (_, c) => ({
      q: starts[r] + c,
      r,
      x: Math.sqrt(3) * (starts[r] + c + r / 2),
      y: 1.5 * r,
    })),
  );
  const min = Math.min(...raw.map((p) => p.x)),
    max = Math.max(...raw.map((p) => p.x));
  const size = Math.min(580 / (max - min + Math.sqrt(3)), 480 / 11);
  const points = raw.map((p) => ({
    x: 340 + (p.x - (min + max) / 2) * size,
    y: 290 + (p.y - 4.5) * size,
  }));
  const vertex = (i: number, k: number) => {
    const a = ((30 + k * 60) * Math.PI) / 180,
      p = points[i];
    return `${(p.x + size * Math.cos(a)).toFixed(3)},${(p.y + size * Math.sin(a)).toFixed(3)}`;
  };
  const neighbors = new Map<string, Set<string>>();
  for (let i = 0; i < points.length; i++)
    for (let k = 0; k < 6; k++) {
      const a = vertex(i, k),
        b = vertex(i, (k + 1) % 6);
      if (!neighbors.has(a)) neighbors.set(a, new Set());
      if (!neighbors.has(b)) neighbors.set(b, new Set());
      neighbors.get(a)!.add(b);
      neighbors.get(b)!.add(a);
    }
  const center = raw[Math.floor(raw.length / 2)];
  const distance = (i: number) => {
    const q = raw[i].q - center.q,
      r = raw[i].r - center.r;
    return q * q + q * r + r * r;
  };
  return map.hexes
    .flatMap((h, i) =>
      h.resource === 6 &&
      [1, 3, 5].every((k) => neighbors.get(vertex(i, k))?.size === 3)
        ? [i]
        : [],
    )
    .sort((a, b) => distance(a) - distance(b) || a - b)
    .slice(0, map.hexes.length === 63 ? 2 : 1);
}
