export type Point = { x: number; y: number };
export type Size = { width: number; height: number };
export type Camera = Point & { scale: number };
export type FingerPair = { a: Point; b: Point };

const fitScale = (map: Size, viewport: Size) =>
  Math.max(
    0.001,
    Math.min(
      Math.max(1, viewport.width) / Math.max(1, map.width),
      Math.max(1, viewport.height) / Math.max(1, map.height),
    ),
  );
const clamp = (camera: Camera, map: Size, viewport: Size): Camera => {
  const width = map.width * camera.scale,
    height = map.height * camera.scale;
  return {
    scale: camera.scale,
    x:
      width <= viewport.width
        ? (viewport.width - width) / 2
        : Math.max(viewport.width - width, Math.min(0, camera.x)),
    y:
      height <= viewport.height
        ? (viewport.height - height) / 2
        : Math.max(viewport.height - height, Math.min(0, camera.y)),
  };
};
export const fitCamera = (map: Size, viewport: Size): Camera =>
  clamp({ x: 0, y: 0, scale: fitScale(map, viewport) }, map, viewport);
export const mapPoint = (camera: Camera, point: Point): Point => ({
  x: (point.x - camera.x) / camera.scale,
  y: (point.y - camera.y) / camera.scale,
});
export const viewPoint = (camera: Camera, point: Point): Point => ({
  x: point.x * camera.scale + camera.x,
  y: point.y * camera.scale + camera.y,
});
export function zoomCamera(
  camera: Camera,
  factor: number,
  anchor: Point,
  map: Size,
  viewport: Size,
): Camera {
  const min = fitScale(map, viewport),
    scale = Math.max(min, Math.min(min * 8, camera.scale * factor));
  const point = mapPoint(camera, anchor);
  return clamp(
    { scale, x: anchor.x - point.x * scale, y: anchor.y - point.y * scale },
    map,
    viewport,
  );
}
export const panCamera = (
  camera: Camera,
  x: number,
  y: number,
  map: Size,
  viewport: Size,
): Camera =>
  clamp({ ...camera, x: camera.x + x, y: camera.y + y }, map, viewport);
export const locateCamera = (
  camera: Camera,
  point: Point,
  map: Size,
  viewport: Size,
): Camera =>
  clamp(
    {
      ...camera,
      x: viewport.width / 2 - point.x * camera.scale,
      y: viewport.height / 2 - point.y * camera.scale,
    },
    map,
    viewport,
  );
export function pinchCamera(
  camera: Camera,
  start: FingerPair,
  end: FingerPair,
  map: Size,
  viewport: Size,
): Camera {
  const midpoint = (p: FingerPair) => ({
    x: (p.a.x + p.b.x) / 2,
    y: (p.a.y + p.b.y) / 2,
  });
  const distance = (p: FingerPair) => Math.hypot(p.a.x - p.b.x, p.a.y - p.b.y);
  const before = midpoint(start),
    after = midpoint(end),
    origin = mapPoint(camera, before);
  const min = fitScale(map, viewport),
    factor = distance(start) > 0 ? distance(end) / distance(start) : 1;
  const scale = Math.max(min, Math.min(min * 8, camera.scale * factor));
  return clamp(
    { scale, x: after.x - origin.x * scale, y: after.y - origin.y * scale },
    map,
    viewport,
  );
}
