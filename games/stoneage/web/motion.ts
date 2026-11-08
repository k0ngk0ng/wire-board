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
};
export function impactOffset(event: { duration: number; impactAt?: number }) {
  return event.impactAt ?? event.duration / 2;
}
export function sampleAttackMotion(motion: AttackMotion, elapsed: number) {
  const time = Math.max(0, elapsed);
  const returnStart =
    motion.attackStart +
    motion.attackDuration +
    motion.contactHold +
    motion.postHold;
  let amount = 1,
    action = 0,
    progress: number | undefined,
    phase = "attack";
  if (time < motion.approach) {
    amount = time / Math.max(1, motion.approach);
    action = 4;
    phase = "approach";
  } else if (time < motion.attackStart) {
    action = 3;
    phase = "ready";
  } else if (time >= returnStart) {
    amount = Math.max(
      0,
      1 - (time - returnStart) / Math.max(1, motion.returnDuration),
    );
    action = 4;
    phase = "return";
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
    offset:
      amount === 0
        ? [0, 0]
        : [
            (motion.to[0] - motion.from[0]) * amount,
            (motion.to[1] - motion.from[1]) * amount,
          ],
    action,
    progress,
    phase,
  };
}
