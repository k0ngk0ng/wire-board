import type { Shot, Fall } from "./types";
import { renderableShot } from "./timeline.ts";
export type TankPoseInput = {
  alive: boolean;
  seat: number;
  now: number;
  shot?: Pick<Shot, "seat" | "at" | "launchAt" | "cancelledAt">;
  fall?: Pick<Fall, "at" | "endAt">;
  motionAt?: number;
  damageAt?: number;
  itemAt?: number;
  deathAt?: number;
  actions?: Record<
    string,
    { durationMs: number; right?: number[]; left?: number[] }
  >;
  reduced: boolean;
};
export function tankPose(input: TankPoseInput): {
  action: string;
  elapsed: number;
} {
  const {
    alive,
    seat,
    now,
    fall,
    motionAt,
    damageAt,
    itemAt,
    deathAt,
    actions,
    reduced,
  } = input;
  const shot = renderableShot(input.shot);
  if (!alive) {
    const elapsed = deathAt === undefined ? 10000 : now - deathAt;
    const duration = actions?.death?.durationMs ?? 700;
    return actions?.dead && elapsed >= duration
      ? { action: "dead", elapsed: elapsed - duration }
      : { action: "death", elapsed };
  }
  // The launch frame itself must remain visible, not only the preparation.
  // Release priority after that one frame so early impacts still animate falls.
  const fireFrameMs = Math.ceil(
    (actions?.fire?.durationMs ?? 667) /
      Math.max(
        1,
        actions?.fire?.right?.length ?? actions?.fire?.left?.length ?? 10,
      ),
  );
  if (
    shot?.seat === seat &&
    shot.launchAt !== undefined &&
    now >= shot.at &&
    now < shot.launchAt + fireFrameMs
  )
    return { action: "fire", elapsed: now - shot.at };
  if (fall && now < fall.endAt)
    return { action: "fall", elapsed: now - fall.at };
  if (
    damageAt !== undefined &&
    now >= damageAt &&
    now - damageAt < (actions?.damage?.durationMs ?? 400)
  )
    return { action: "damage", elapsed: now - damageAt };
  if (motionAt !== undefined && now - motionAt < 180 && !reduced)
    return { action: "move", elapsed: now };
  if (shot?.seat === seat && now >= shot.at) {
    const elapsed = now - shot.at,
      fire = actions?.fire?.durationMs ?? 500,
      after = actions?.afterFire?.durationMs ?? 0;
    if (elapsed < fire) return { action: "fire", elapsed };
    if (after > 0 && elapsed < fire + after)
      return { action: "afterFire", elapsed: elapsed - fire };
  }
  if (
    itemAt !== undefined &&
    now >= itemAt &&
    now - itemAt < (actions?.item?.durationMs ?? 667)
  )
    return { action: "item", elapsed: now - itemAt };
  return { action: "idle", elapsed: now };
}
