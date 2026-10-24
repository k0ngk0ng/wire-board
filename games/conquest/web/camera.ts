export type Point = { x: number; y: number };
export type Size = { width: number; height: number };
export type Camera = Point & { scale: number };
export type FingerPair = { a: Point; b: Point };
export const CAMERA_MAX_RATIO = 32;
export const CAMERA_PAN_PADDING = 0.25;

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
  const axis = (position: number, extent: number, available: number) => {
    const padding = available * CAMERA_PAN_PADDING;
    const centre = (available - extent) / 2;
    const min =
      extent <= available ? centre - padding : available - extent - padding;
    const max = extent <= available ? centre + padding : padding;
    return Math.max(min, Math.min(max, position));
  };
  return {
    scale: camera.scale,
    x: axis(camera.x, width, viewport.width),
    y: axis(camera.y, height, viewport.height),
  };
};
export const fitCamera = (map: Size, viewport: Size): Camera => {
  const scale = fitScale(map, viewport);
  return {
    x: (viewport.width - map.width * scale) / 2,
    y: (viewport.height - map.height * scale) / 2,
    scale,
  };
};
export function visibleCameraRect(camera: Camera, map: Size, viewport: Size) {
  const x = Math.max(0, Math.min(map.width, -camera.x / camera.scale));
  const y = Math.max(0, Math.min(map.height, -camera.y / camera.scale));
  const right = Math.max(
    0,
    Math.min(map.width, (viewport.width - camera.x) / camera.scale),
  );
  const bottom = Math.max(
    0,
    Math.min(map.height, (viewport.height - camera.y) / camera.scale),
  );
  return {
    x,
    y,
    width: Math.max(0, right - x),
    height: Math.max(0, bottom - y),
  };
}
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
    scale = Math.max(
      min,
      Math.min(min * CAMERA_MAX_RATIO, camera.scale * factor),
    );
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
export function revealCamera(
  camera: Camera,
  points: Point[],
  map: Size,
  viewport: Size,
): Camera {
  const valid = points.filter(
    (p) => Number.isFinite(p.x) && Number.isFinite(p.y),
  );
  if (!valid.length) return camera;
  const side = Math.min(48, viewport.width / 6),
    top = Math.min(128, viewport.height / 3),
    bottom = Math.min(85, viewport.height / 4);
  if (
    valid.every((p) => {
      const v = viewPoint(camera, p);
      return (
        v.x >= side &&
        v.x <= viewport.width - side &&
        v.y >= top &&
        v.y <= viewport.height - bottom
      );
    })
  )
    return camera;
  const left = Math.min(...valid.map((p) => p.x)),
    right = Math.max(...valid.map((p) => p.x));
  const upper = Math.min(...valid.map((p) => p.y)),
    lower = Math.max(...valid.map((p) => p.y));
  const scale = Math.max(
    fitScale(map, viewport),
    Math.min(
      camera.scale,
      (viewport.width - 2 * side) / Math.max(1, right - left),
      (viewport.height - top - bottom) / Math.max(1, lower - upper),
    ),
  );
  return clamp(
    {
      scale,
      x: viewport.width / 2 - ((left + right) * scale) / 2,
      y:
        top +
        (viewport.height - top - bottom) / 2 -
        ((upper + lower) * scale) / 2,
    },
    map,
    viewport,
  );
}
export function restoreCamera(
  camera: Camera,
  previous: Size,
  map: Size,
  viewport: Size,
): Camera {
  const centre = mapPoint(camera, {
    x: previous.width / 2,
    y: previous.height / 2,
  });
  return locateCamera(
    {
      ...camera,
      scale: (camera.scale * fitScale(map, viewport)) / fitScale(map, previous),
    },
    centre,
    map,
    viewport,
  );
}
export function focusCamera(
  camera: Camera,
  point: Point,
  map: Size,
  viewport: Size,
): Camera {
  const fit = fitScale(map, viewport);
  const scale = Math.min(
    fit * CAMERA_MAX_RATIO,
    Math.max(
      camera.scale,
      fit * 2.5,
      (Math.max(1, viewport.height) / Math.max(1, map.height)) * 1.05,
    ),
  );
  return locateCamera({ ...camera, scale }, point, map, viewport);
}
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
  const scale = Math.max(
    min,
    Math.min(min * CAMERA_MAX_RATIO, camera.scale * factor),
  );
  return clamp(
    { scale, x: after.x - origin.x * scale, y: after.y - origin.y * scale },
    map,
    viewport,
  );
}
