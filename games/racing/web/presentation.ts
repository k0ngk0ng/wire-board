import type { Car, Point } from "./types";
export type Camera = { x: number; y: number; scale: number };
export type Viewport = { width: number; height: number };
export const clamp = (n: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, n));
export const angleDelta = (from: number, to: number) =>
  Math.atan2(Math.sin(to - from), Math.cos(to - from));
export function screenToWorld(c: Camera, v: Viewport, p: Point): Point {
  return {
    x: c.x + (p.x - v.width / 2) / c.scale,
    y: c.y + (p.y - v.height / 2) / c.scale,
  };
}
export function zoomAt(
  c: Camera,
  v: Viewport,
  p: Point,
  factor: number,
): Camera {
  const point = screenToWorld(c, v, p),
    scale = clamp(c.scale * factor, 0.05, 2.4);
  return {
    x: point.x - (p.x - v.width / 2) / scale,
    y: point.y - (p.y - v.height / 2) / scale,
    scale,
  };
}
export const panBy = (c: Camera, dx: number, dy: number): Camera => ({
  ...c,
  x: c.x - dx / c.scale,
  y: c.y - dy / c.scale,
});
export function interpolateCar(a: Car, b: Car, t: number): Car {
  if (Math.hypot(a.x - b.x, a.y - b.y) > 160 || a.resetUntil !== b.resetUntil)
    return { ...b };
  t = clamp(t, 0, 1);
  return {
    ...b,
    x: a.x + (b.x - a.x) * t,
    y: a.y + (b.y - a.y) * t,
    angle: a.angle + angleDelta(a.angle, b.angle) * t,
  };
}
export function extrapolateCar(c: Car, ms: number): Car {
  if (c.finishAt) return { ...c };
  const dt = clamp(ms, 0, 250) / 1000;
  return { ...c, x: c.x + c.vx * dt, y: c.y + c.vy * dt };
}
