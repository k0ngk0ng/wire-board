import type { Vec } from "./types";
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
export const WORLD_SCALE = 2.75;
export const paddleRadiusPixels = (width: number) => (width < 650 ? 24 : 42);
export function projectToScreen(point: Vec, width: number, height: number) {
  const c = cameraSetup(width, height),
    pitch = Math.atan2(c.y - c.targetY, c.z + 1.1),
    t = Math.tan((c.fov * Math.PI) / 360);
  const dy = point.y * WORLD_SCALE - c.y,
    dz = point.z * WORLD_SCALE - c.z;
  const depth = -dy * Math.sin(pitch) - dz * Math.cos(pitch);
  return {
    x:
      ((point.x * WORLD_SCALE) / ((depth * t * width) / Math.max(1, height)) +
        1) /
      2,
    y: (1 - (dy * Math.cos(pitch) - dz * Math.sin(pitch)) / (depth * t)) / 2,
    depth,
  };
}
export function pointerOverBall(
  pointer: { x: number; y: number },
  ball: Vec,
  width: number,
  height: number,
): boolean {
  const at = projectToScreen(ball, width, height),
    radius = paddleRadiusPixels(width);
  return (
    at.depth > 0 &&
    Math.hypot(
      ((pointer.x - at.x) * width) / (radius + 10),
      ((pointer.y - at.y) * height) / (radius * 1.13 + 10),
    ) <= 1
  );
}
export function paddleReachRadius(
  pointer: { x: number; y: number },
  z: number,
  width: number,
  height: number,
): number {
  const centre = paddleAtScreen(pointer, z, width, height),
    r = paddleRadiusPixels(width);
  let reach = 0.04;
  for (let i = 0; i < 8; i++) {
    const angle = (i * Math.PI) / 4,
      edge = paddleAtScreen(
        {
          x: pointer.x + (Math.cos(angle) * (r + 10)) / width,
          y: pointer.y + (Math.sin(angle) * (r * 1.13 + 10)) / height,
        },
        z,
        width,
        height,
      );
    reach = Math.max(reach, Math.hypot(edge.x - centre.x, edge.y - centre.y));
  }
  return Math.min(0.45, reach);
}
export function cameraSetup(width: number, _height: number) {
  return width < 650
    ? { y: 4.8, z: 12, fov: 72, targetY: 2.1 }
    : { y: 4.3, z: 9.2, fov: 54, targetY: 2.1 };
}
/** Intersect the pointer ray with the current paddle depth; never clamp X/Y. */
export function paddleAtScreen(
  point: { x: number; y: number },
  z: number,
  width: number,
  height: number,
): Vec {
  const c = cameraSetup(width, height),
    pitch = Math.atan2(c.y - c.targetY, c.z + 1.1);
  const tangent = Math.tan((c.fov * Math.PI) / 360),
    up = (1 - 2 * point.y) * tangent;
  const ray = {
    x: ((2 * point.x - 1) * tangent * width) / Math.max(1, height),
    y: -Math.sin(pitch) + up * Math.cos(pitch),
    z: -Math.cos(pitch) - up * Math.sin(pitch),
  };
  const distance = (z * WORLD_SCALE - c.z) / ray.z;
  return {
    x: (ray.x * distance) / WORLD_SCALE,
    y: (c.y + ray.y * distance) / WORLD_SCALE,
    z,
  };
}
/** Horizontal finger position projected onto the anticipated contact plane.
 * The finger can start at any vertical location; no tiny ball target is needed.
 */
export function racketFromScreen(
  screenX: number,
  localContact: Vec,
  width: number,
  height: number,
): number {
  const c = cameraSetup(width, height),
    pitch = Math.atan2(c.y - c.targetY, c.z + 1.1);
  const depth =
    (c.z - localContact.z * WORLD_SCALE) * Math.cos(pitch) +
    (c.y - localContact.y * WORLD_SCALE) * Math.sin(pitch);
  const worldX =
    (screenX *
      Math.max(0.1, depth) *
      Math.tan((c.fov * Math.PI) / 360) *
      Math.max(1, width)) /
    Math.max(1, height);
  return clamp(worldX / WORLD_SCALE / 0.9, -1, 1);
}
