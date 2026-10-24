import type { Vec } from "./types";
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
export const WORLD_SCALE = 2.75;
export function cameraSetup(width: number, _height: number) {
  return width < 650
    ? { y: 4.8, z: 9.5, fov: 68, targetY: 2.1 }
    : { y: 4.3, z: 8.2, fov: 54, targetY: 2.1 };
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
