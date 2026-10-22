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

/** Touch-only floating origin; a second pointer is filtered by the owning UI. */
export class TouchStick {
  private origin?: { x: number; y: number; radius: number };
  begin(x: number, y: number, radius: number) {
    this.origin =
      [x, y, radius].every(Number.isFinite) && radius > 0
        ? { x, y, radius }
        : undefined;
  }
  reset() {
    this.origin = undefined;
  }
  move(x: number, y: number, previous: Move) {
    const origin = this.origin;
    if (!origin || !Number.isFinite(x) || !Number.isFinite(y))
      return { x: 0, y: 0, move: "none" as Move };
    const dx = x - origin.x,
      dy = y - origin.y;
    const intent = joystickIntent(dx, dy, origin.radius, previous);
    const distance = Math.hypot(dx, dy);
    if (distance > origin.radius) {
      origin.x = x - (dx * origin.radius) / distance;
      origin.y = y - (dy * origin.radius) / distance;
    }
    return intent;
  }
}
