import type { Camera, Point } from "./aim";
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
export function boundCamera(c: Camera, width: number, height: number): Camera {
  return {
    ...c,
    x: clamp(c.x, 0, Math.max(0, width - c.worldViewW)),
    y: clamp(c.y, -320, Math.max(-320, height - c.worldViewH)),
  };
}
export function centeredCamera(
  screenW: number,
  screenH: number,
  worldViewW: number,
  target: Point,
  width: number,
  height: number,
): Camera {
  const worldViewH = (worldViewW * screenH) / Math.max(1, screenW);
  return boundCamera(
    {
      x: target.x - worldViewW / 2,
      y: target.y - worldViewH * 0.62,
      worldViewW,
      worldViewH,
    },
    width,
    height,
  );
}
export function zoomAt(
  c: Camera,
  factor: number,
  anchor: Point,
  width: number,
  height: number,
): Camera {
  const w = clamp(c.worldViewW * factor, 240, Math.min(1200, width));
  const h = (c.worldViewH * w) / c.worldViewW;
  return boundCamera(
    {
      x: c.x + (c.worldViewW - w) * anchor.x,
      y: c.y + (c.worldViewH - h) * anchor.y,
      worldViewW: w,
      worldViewH: h,
    },
    width,
    height,
  );
}
export function panCamera(
  c: Camera,
  dx: number,
  dy: number,
  width: number,
  height: number,
): Camera {
  // Manual vertical panning keeps map bounds; automatic tracking may show sky.
  return {
    ...boundCamera({ ...c, x: c.x + dx, y: c.y + dy }, width, height),
    y: clamp(c.y + dy, 0, Math.max(0, height - c.worldViewH)),
  };
}
