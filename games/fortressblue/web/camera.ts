import type { Camera, Point } from "./aim";
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
function fittedView(
  worldViewW: number,
  worldViewH: number,
  width: number,
  height: number,
) {
  const scale = Math.min(1, width / worldViewW, height / worldViewH);
  return { worldViewW: worldViewW * scale, worldViewH: worldViewH * scale };
}
export function boundCamera(c: Camera, width: number, height: number): Camera {
  const view = fittedView(c.worldViewW, c.worldViewH, width, height);
  return {
    ...c,
    ...view,
    x: clamp(c.x, 0, Math.max(0, width - view.worldViewW)),
    // Shells keep their authoritative coordinates above the original image;
    // the camera stops at its top edge and naturally follows them on return.
    y: clamp(c.y, 0, Math.max(0, height - view.worldViewH)),
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
  const view = fittedView(
    worldViewW,
    (worldViewW * screenH) / Math.max(1, screenW),
    width,
    height,
  );
  return boundCamera(
    {
      x: target.x - view.worldViewW / 2,
      y: target.y - view.worldViewH * 0.62,
      ...view,
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
  const maxWidth = Math.min(1200, width, (height * c.worldViewW) / c.worldViewH);
  const w = clamp(c.worldViewW * factor, Math.min(240, maxWidth), maxWidth);
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
  return boundCamera({ ...c, x: c.x + dx, y: c.y + dy }, width, height);
}
