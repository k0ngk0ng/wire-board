import type { Player, Point } from "./types.ts";
const clamp = (n: number, a: number, b: number) => Math.max(a, Math.min(b, n));

// Presentation only: teammates retain their authoritative interaction points.
// The viewer stays anchored while nearby people yield a little visual space.
export function spreadCharacters(
  players: readonly Pick<Player, "seat" | "room">[],
  points: ReadonlyMap<number, Point>,
  viewer: number,
): Map<number, Point> {
  const result = new Map(points);
  const ordered = [...players].sort((a, b) => a.seat - b.seat);
  const seen = new Set<number>();
  for (const first of ordered) {
    if (seen.has(first.seat)) continue;
    const group = [first];
    seen.add(first.seat);
    for (let i = 0; i < group.length; i++) {
      const a = group[i],
        p = points.get(a.seat);
      if (!p) continue;
      for (const b of ordered) {
        const q = points.get(b.seat);
        if (
          !seen.has(b.seat) &&
          a.room === b.room &&
          q &&
          Math.hypot(p.x - q.x, p.y - q.y) < 28
        ) {
          seen.add(b.seat);
          group.push(b);
        }
      }
    }
    if (group.length < 2) continue;
    group.sort((a, b) => a.seat - b.seat);
    const self = group.find((p) => p.seat === viewer);
    let base: Point;
    let slots: Point[];
    if (self) {
      base = points.get(self.seat)!;
      const x = base.x < 500 ? 40 : -40,
        y = base.y < 400 ? 42 : -42;
      slots = [
        { x, y: 0 },
        { x: 0, y },
        { x, y },
      ];
      let n = 0;
      for (const p of group) {
        if (p.seat === viewer) continue;
        const offset = slots[n++];
        result.set(p.seat, {
          x: clamp(base.x + offset.x, 145, 855),
          y: clamp(base.y + offset.y, 270, 565),
        });
      }
    } else {
      const values = group.map((p) => points.get(p.seat)!);
      const cx = values.reduce((n, p) => n + p.x, 0) / values.length,
        cy = values.reduce((n, p) => n + p.y, 0) / values.length;
      slots =
        group.length === 2
          ? [
              { x: -30, y: 0 },
              { x: 30, y: 0 },
            ]
          : group.length === 3
            ? [
                { x: -38, y: 0 },
                { x: 0, y: -24 },
                { x: 38, y: 0 },
              ]
            : [
                { x: -30, y: -24 },
                { x: 30, y: -24 },
                { x: -30, y: 24 },
                { x: 30, y: 24 },
              ];
      const ex = Math.max(...slots.map((p) => Math.abs(p.x))),
        ey = Math.max(...slots.map((p) => Math.abs(p.y)));
      base = {
        x: clamp(cx, 145 + ex, 855 - ex),
        y: clamp(cy, 270 + ey, 565 - ey),
      };
      group.forEach((p, i) =>
        result.set(p.seat, { x: base.x + slots[i].x, y: base.y + slots[i].y }),
      );
    }
  }
  return result;
}
