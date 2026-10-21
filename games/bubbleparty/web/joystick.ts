import type { Move } from "./input";
/** A free-moving thumb control that preserves classic cardinal grid movement. */
export function joystickIntent(
  dx: number,
  dy: number,
  radius: number,
  previous: Move,
): { x: number; y: number; move: Move } {
  if (
    !Number.isFinite(dx) ||
    !Number.isFinite(dy) ||
    !Number.isFinite(radius) ||
    radius <= 0
  )
    return { x: 0, y: 0, move: "none" };
  const distance = Math.hypot(dx, dy);
  if (distance < radius * 0.17) return { x: 0, y: 0, move: "none" };
  const scale = Math.max(radius, distance),
    x = dx / scale,
    y = dy / scale;
  let horizontal = Math.abs(dx) >= Math.abs(dy);
  if (
    (previous === "left" || previous === "right") &&
    Math.abs(dy) < Math.abs(dx) * 1.18
  )
    horizontal = true;
  if (
    (previous === "up" || previous === "down") &&
    Math.abs(dx) < Math.abs(dy) * 1.18
  )
    horizontal = false;
  return {
    x,
    y,
    move: horizontal ? (dx < 0 ? "left" : "right") : dy < 0 ? "up" : "down",
  };
}
