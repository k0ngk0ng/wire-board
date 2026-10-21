export type SlopeFrame = { sourceIndex: number; facing: string };
/** Visual pose only. This never changes the server position or muzzle angle. */
export function supportSlope(
  tank: { x: number; y: number },
  solid: (x: number, y: number) => boolean,
) {
  const support = (x: number) => {
    let nearest: number | null = null;
    for (let y = Math.floor(tank.y) - 24; y <= Math.ceil(tank.y) + 24; y++) {
      if (
        solid(x, y) &&
        !solid(x, y - 1) &&
        (nearest === null || Math.abs(y - tank.y) < Math.abs(nearest - tank.y))
      )
        nearest = y;
    }
    return nearest;
  };
  const left = support(tank.x - 12),
    right = support(tank.x + 12);
  // A cliff/crater or detached fall has no pair of nearby supporting surfaces.
  if (left === null || right === null) return { slope: 0, left, right };
  const angle = (Math.atan2(right - left, 24) * 180) / Math.PI;
  return {
    slope: Math.max(-2, Math.min(2, Math.round(angle / 22.5))),
    left,
    right,
  };
}
/** Native atlases interleave both original directions; retain the exact anchors. */
export function selectSlopeFrame<T extends SlopeFrame>(
  frames: T[],
  baseIndex: number,
  worldSlope: number,
  facing: number,
  slopeVariants: number,
): T | undefined {
  const base = frames[baseIndex];
  if (!base || slopeVariants !== 5) return base;
  // Canvas y grows downward. Primary (left-facing) source 0 slopes down
  // toward screen-right; alternate (right-facing) source 4 does the same.
  const slope =
    Math.max(-2, Math.min(2, Math.round(worldSlope))) * (facing < 0 ? -1 : 1);
  const candidate = frames[baseIndex + 2 * slope];
  if (
    !candidate ||
    candidate.facing !== base.facing ||
    candidate.sourceIndex !== base.sourceIndex + slope
  )
    return base;
  return candidate;
}
