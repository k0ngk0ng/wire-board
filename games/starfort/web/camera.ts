import type { Point } from "./types.ts";
export type Viewport = { width: number; height: number };
export type WorldBounds = Point & Viewport;
export type Camera = Point & { zoom: number };
const safe = (value: number, fallback: number) =>
  Number.isFinite(value) ? value : fallback;
export function worldBounds(map: Viewport): WorldBounds {
  // Headroom belongs to the world, so large tower artwork retains its true pad.
  const ratio = Math.max(1, map.width / 960);
  const side = 64 * ratio,
    top = 180 * ratio;
  return {
    x: -side,
    y: -top,
    width: Math.max(1, map.width) + side * 2,
    height: Math.max(1, map.height) + top + side,
  };
}
export function cameraWindow(camera: Camera, viewport: Viewport): WorldBounds {
  const width = Math.max(1, viewport.width) / camera.zoom;
  const height = Math.max(1, viewport.height) / camera.zoom;
  return { x: camera.x - width / 2, y: camera.y - height / 2, width, height };
}
export function fitCamera(bounds: WorldBounds, viewport: Viewport): Camera {
  return {
    x: bounds.x + bounds.width / 2,
    y: bounds.y + bounds.height / 2,
    zoom: Math.max(
      0.02,
      Math.min(
        Math.max(1, viewport.width) / bounds.width,
        Math.max(1, viewport.height) / bounds.height,
        2,
      ),
    ),
  };
}
export function clampCamera(
  camera: Camera,
  bounds: WorldBounds,
  viewport: Viewport,
): Camera {
  const fit = fitCamera(bounds, viewport);
  const zoom = Math.max(fit.zoom, Math.min(2, safe(camera.zoom, fit.zoom)));
  const halfWidth = Math.max(1, viewport.width) / zoom / 2;
  const halfHeight = Math.max(1, viewport.height) / zoom / 2;
  return {
    x:
      halfWidth * 2 >= bounds.width
        ? fit.x
        : Math.max(
            bounds.x + halfWidth,
            Math.min(
              bounds.x + bounds.width - halfWidth,
              safe(camera.x, fit.x),
            ),
          ),
    y:
      halfHeight * 2 >= bounds.height
        ? fit.y
        : Math.max(
            bounds.y + halfHeight,
            Math.min(
              bounds.y + bounds.height - halfHeight,
              safe(camera.y, fit.y),
            ),
          ),
    zoom,
  };
}
export function worldToScreen(
  point: Point,
  camera: Camera,
  viewport: Viewport,
): Point {
  return {
    x: (point.x - camera.x) * camera.zoom + viewport.width / 2,
    y: (point.y - camera.y) * camera.zoom + viewport.height / 2,
  };
}
export function screenToWorld(
  point: Point,
  camera: Camera,
  viewport: Viewport,
): Point {
  return {
    x: (point.x - viewport.width / 2) / camera.zoom + camera.x,
    y: (point.y - viewport.height / 2) / camera.zoom + camera.y,
  };
}
export function panCamera(
  camera: Camera,
  delta: Point,
  bounds: WorldBounds,
  viewport: Viewport,
): Camera {
  return clampCamera(
    {
      ...camera,
      x: camera.x - delta.x / camera.zoom,
      y: camera.y - delta.y / camera.zoom,
    },
    bounds,
    viewport,
  );
}
export function zoomCamera(
  camera: Camera,
  factor: number,
  point: Point,
  bounds: WorldBounds,
  viewport: Viewport,
): Camera {
  const anchor = screenToWorld(point, camera, viewport);
  const zoom = Math.max(
    fitCamera(bounds, viewport).zoom,
    Math.min(2, safe(camera.zoom * factor, camera.zoom)),
  );
  return clampCamera(
    {
      x: anchor.x - (point.x - viewport.width / 2) / zoom,
      y: anchor.y - (point.y - viewport.height / 2) / zoom,
      zoom,
    },
    bounds,
    viewport,
  );
}
export function focusCamera(
  camera: Camera,
  point: Point,
  bounds: WorldBounds,
  viewport: Viewport,
): Camera {
  return clampCamera({ ...camera, x: point.x, y: point.y }, bounds, viewport);
}
export function dragMoved(start: Point, end: Point): boolean {
  return Math.hypot(start.x - end.x, start.y - end.y) > 6;
}

// Full-map views describe the battlefield. Construction remains in detail mode
// where each pad can retain its minimum-size target without covering its peers.
export function isWorldOverview(
  map: Viewport,
  viewport: Viewport,
  camera: Camera,
): boolean {
  if (
    !viewport.width ||
    !viewport.height ||
    (map.width <= 960 && map.height <= 640)
  )
    return false;
  const starter = fitCamera(worldBounds({ width: 960, height: 640 }), viewport);
  return camera.zoom < starter.zoom * 0.84;
}
export function detailCameraAt(
  camera: Camera,
  point: Point,
  map: Viewport,
  viewport: Viewport,
): Camera {
  const starter = fitCamera(worldBounds({ width: 960, height: 640 }), viewport);
  return focusCamera(
    { ...camera, zoom: Math.max(camera.zoom, starter.zoom, 0.35) },
    point,
    worldBounds(map),
    viewport,
  );
}
