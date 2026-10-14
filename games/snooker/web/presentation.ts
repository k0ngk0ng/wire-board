import type { Ball, Flight, Vec } from "./types";
export const W = 3.569,
  H = 1.778,
  R = 0.02625,
  BAULK = 0.737,
  D = 0.292;
export type Camera = {
  width: number;
  height: number;
  portrait: boolean;
  zoom: number;
  pan: Vec;
};
export function scaleOf(c: Camera) {
  return (
    Math.min(
      c.width / (c.portrait ? H + 0.38 : W + 0.38),
      c.height / (c.portrait ? W + 0.38 : H + 0.38),
    ) * c.zoom
  );
}
export function worldToScreen(p: Vec, c: Camera): Vec {
  const x = p.x - W / 2,
    y = p.y - H / 2,
    k = scaleOf(c);
  return {
    x: c.width / 2 + c.pan.x + (c.portrait ? -y : x) * k,
    y: c.height / 2 + c.pan.y + (c.portrait ? x : y) * k,
  };
}
export function screenToWorld(p: Vec, c: Camera): Vec {
  const k = scaleOf(c),
    x = (p.x - c.width / 2 - c.pan.x) / k,
    y = (p.y - c.height / 2 - c.pan.y) / k;
  return { x: W / 2 + (c.portrait ? y : x), y: H / 2 + (c.portrait ? -x : y) };
}
export function clampPan(c: Camera): Camera {
  const k = scaleOf(c),
    mx = Math.max(0, ((c.portrait ? H + 0.38 : W + 0.38) * k - c.width) / 2),
    my = Math.max(0, ((c.portrait ? W + 0.38 : H + 0.38) * k - c.height) / 2);
  return {
    ...c,
    pan: {
      x: Math.max(-mx, Math.min(mx, c.pan.x)),
      y: Math.max(-my, Math.min(my, c.pan.y)),
    },
  };
}
export function zoomAt(c: Camera, zoom: number, anchor: Vec): Camera {
  const p = screenToWorld(anchor, c),
    next = { ...c, zoom: Math.max(1, Math.min(4, zoom)) },
    after = worldToScreen(p, next);
  return clampPan({
    ...next,
    pan: { x: c.pan.x + anchor.x - after.x, y: c.pan.y + anchor.y - after.y },
  });
}
export function sampleFlight(f: Pick<Flight, "motion">, time: number): Ball[] {
  const frames = f.motion.frames;
  if (!frames.length) return f.motion.balls ?? [];
  let lo = 0,
    hi = frames.length - 1;
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2);
    if (frames[mid].time <= time) lo = mid;
    else hi = mid - 1;
  }
  const a = frames[lo],
    b = frames[Math.min(lo + 1, frames.length - 1)],
    portion =
      b.time > a.time
        ? Math.max(0, Math.min(1, (time - a.time) / (b.time - a.time)))
        : 1;
  const current = new Map<number, Ball>();
  for (let i = 0; i <= lo; i++)
    for (const ball of frames[i].balls) current.set(ball.id, ball);
  return [...current.values()].map((ball) => {
    const next = b.balls.find((v) => v.id === ball.id) ?? ball;
    const pot = f.motion.events.find(
      (e) => (e.kind === "pot" || e.kind === "off") && e.ball === ball.id,
    );
    return {
      ...ball,
      down: ball.down || !!(pot && time >= pot.time),
      pos: {
        x: ball.pos.x + (next.pos.x - ball.pos.x) * portion,
        y: ball.pos.y + (next.pos.y - ball.pos.y) * portion,
      },
    };
  });
}
export function recentImpacts<T extends { time: number }>(
  events: readonly T[],
  from: number,
  to: number,
): T[] {
  return events.filter(
    (e) => e.time > from && e.time <= to && to - e.time <= 0.3,
  );
}
