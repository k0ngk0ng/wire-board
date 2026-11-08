export type AttackMotion = {
  actor: number;
  from: number[];
  to: number[];
  approach: number;
  attackStart: number;
  attackDuration: number;
  contactOffset: number;
  contactHold: number;
  postHold: number;
  returnDuration: number;
  start?: number[];
  stay?: boolean;
  idle?: boolean;
  returnStart?: number;
};
export function impactOffset(event: { duration: number; impactAt?: number }) {
  return event.impactAt ?? event.duration / 2;
}
export function sampleAttackMotion(motion: AttackMotion, elapsed: number) {
  const time = Math.max(0, elapsed);
  const returnStart =
    motion.returnStart ??
    motion.attackStart +
      motion.attackDuration +
      motion.contactHold +
      motion.postHold;
  let amount = 1,
    action = 0,
    progress: number | undefined,
    phase = "attack";
  let point = motion.to;
  const start = motion.start ?? motion.from;
  if (time < motion.approach) {
    amount = time / Math.max(1, motion.approach);
    action = 4;
    phase = "approach";
    point = start.map((v, i) => v + (motion.to[i] - v) * amount);
  } else if (time < motion.attackStart) {
    action = 3;
    phase = "ready";
  } else if (time >= returnStart && !motion.stay) {
    amount = Math.max(
      0,
      1 - (time - returnStart) / Math.max(1, motion.returnDuration),
    );
    action = 4;
    phase = "return";
    point = motion.from.map((v, i) => v + (motion.to[i] - v) * amount);
  } else if (motion.idle || time >= returnStart) {
    action = 3;
    phase = "ready";
  } else {
    const attackElapsed = time - motion.attackStart;
    const held = Math.min(
      Math.max(0, attackElapsed - motion.contactOffset),
      motion.contactHold,
    );
    progress = Math.min(
      1,
      (attackElapsed - held) / Math.max(1, motion.attackDuration),
    );
  }
  return {
    offset: point.map((v, i) => v - motion.from[i]),
    action,
    progress,
    phase,
  };
}

export type Projectile = {
  kind: string;
  actor?: number;
  target?: number;
  from: number[];
  to: number[];
  launchAt: number;
  flightDuration: number;
  impactAt: number;
  lingerDuration: number;
  returnDuration?: number;
};
export function sampleProjectile(projectile: Projectile, elapsed: number) {
  const p = projectile,
    finish = p.impactAt + p.lingerDuration + (p.returnDuration ?? 0);
  if (elapsed < p.launchAt || elapsed >= finish) return null;
  let amount = Math.min(
      1,
      (elapsed - p.launchAt) / Math.max(1, p.impactAt - p.launchAt),
    ),
    phase = "flight";
  if (elapsed >= p.impactAt) {
    phase = "stick";
    if (p.returnDuration && elapsed >= p.impactAt + p.lingerDuration) {
      phase = "return";
      amount = 1 - (elapsed - p.impactAt - p.lingerDuration) / p.returnDuration;
    }
  }
  const x = p.from[0] + (p.to[0] - p.from[0]) * amount,
    y = p.from[1] + (p.to[1] - p.from[1]) * amount;
  const angle = Math.atan2(p.to[1] - p.from[1], p.to[0] - p.from[0]);
  const course =
    ((Math.round(((angle + Math.PI / 2) * 16) / Math.PI) % 32) + 32) % 32;
  return { x, y, course, phase, angle: (angle * 180) / Math.PI };
}
