import type { CatanState } from "./types";
import { fishGroundGeometry } from "./catan-fishing-state.ts";

type Box = { left: number; right: number; top: number; bottom: number };
const box = (x: number, y: number, half: number): Box => ({
  left: x - half,
  right: x + half,
  top: y - half,
  bottom: y + half,
});
const overlaps = (a: Box, b: Box) =>
  a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;

// The port's gameplay edge never moves. If its artwork covers a fishing number
// or another port, find a nearby clear callout; leader lines still name that edge.
export function catanPortLayout(g: CatanState) {
  const sea = g.seafarers,
    h = g.hexSize || 62;
  const ports = [...g.ports, ...(sea?.tribe?.ports || [])].map((port) => {
    const e = g.edges[port.edge],
      a = g.vertices[e.a],
      b = g.vertices[e.b];
    const x = (a.x + b.x) / 2,
      y = (a.y + b.y) / 2;
    const land = sea
      ? g.tiles.find(
          (t) =>
            (e.tiles?.includes(t.id) ||
              (t.vertices.includes(e.a) && t.vertices.includes(e.b))) &&
            t.resource !== 6 &&
            t.resource !== 8,
        )
      : undefined;
    const angle = land
      ? Math.atan2(y - land.y, x - land.x)
      : Math.atan2(y - 290, x - 340);
    const offset = sea ? h * 0.62 : 34;
    return {
      port,
      a,
      b,
      x,
      y,
      angle,
      px: x + offset * Math.cos(angle),
      py: y + offset * Math.sin(angle),
      size: sea ? h * 1.02 : 52,
      unclaimed: !!sea?.tribe?.ports?.some((p) => p.edge === port.edge),
    };
  });
  if (!g.fishing || !sea) return ports;
  const numbers = g.fishing.map.grounds.flatMap((ground) => {
    const p = fishGroundGeometry(g, ground.vertices);
    return p ? [box(p.labelX, p.labelY, h * 0.23)] : [];
  });
  const obstacles = [
    ...numbers,
    ...g.tiles.filter((t) => t.number > 0).map((t) => box(t.x, t.y, h * 0.3)),
    ...(sea.wonders?.markers || []).map((m) => {
      const v = g.vertices[m.vertex];
      return box(v.x, v.y, h * 0.3);
    }),
    ...(sea.cloth?.villages || []).map((village) => {
      const v = g.vertices[village.vertex],
        scale = Math.max(0.64, h / 62);
      return {
        left: v.x - 20 * scale,
        right: v.x + 65 * scale,
        top: v.y - 20 * scale,
        bottom: v.y + 20 * scale,
      };
    }),
  ];
  const bounds = {
    left: Math.min(...g.vertices.map((v) => v.x)) - 45,
    right: Math.max(...g.vertices.map((v) => v.x)) + 45,
    top: Math.min(...g.vertices.map((v) => v.y)) - 45,
    bottom: Math.max(...g.vertices.map((v) => v.y)) + 56,
  };
  const original = ports.map((p) => box(p.px, p.py, p.size / 2));
  ports.forEach((p, index) => {
    if (
      !numbers.some((n) => overlaps(original[index], n)) &&
      !original.some(
        (other, i) => i !== index && overlaps(original[index], other),
      )
    )
      return;
    // Earlier callouts have already moved. Their vacated positions must not
    // block a later port in a narrow inlet shared with fishing numbers.
    const occupied = ports.flatMap((other, i) =>
      i === index ? [] : [box(other.px, other.py, other.size / 2)],
    );
    const blocked = [...obstacles, ...occupied];
    const candidates = [];
    // A narrow Wonders inlet can have no full-size slot. Reduce only that
    // callout before moving it so far that its coastal association is lost.
    for (const scale of [1, 0.85, 0.7]) {
      for (let dx = -12; dx <= 12; dx++)
        for (let dy = -12; dy <= 12; dy++) {
          const x = p.px + (dx * h) / 8,
            y = p.py + (dy * h) / 8;
          // Keep the callout on the water side of its actual coastal edge.
          if (
            (x - p.x) * Math.cos(p.angle) + (y - p.y) * Math.sin(p.angle) <
            h * 0.4
          )
            continue;
          const b = box(x, y, (p.size * scale) / 2);
          if (
            b.left < bounds.left ||
            b.right > bounds.right ||
            b.top < bounds.top ||
            b.bottom > bounds.bottom ||
            blocked.some((o) => overlaps(b, o))
          )
            continue;
          candidates.push({
            x,
            y,
            size: p.size * scale,
            distance: dx * dx + dy * dy,
          });
        }
      if (candidates.length) break;
    }
    candidates.sort((a, b) => a.distance - b.distance);
    if (candidates.length) {
      p.px = candidates[0].x;
      p.py = candidates[0].y;
      p.size = candidates[0].size;
    }
  });
  return ports;
}
