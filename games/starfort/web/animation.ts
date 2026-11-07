// Cosmetic poses never change world coordinates, collision or damage.
export function shotPose(kind: string, age: number, reduced: boolean) {
  const pulse =
    reduced || age < 0 || age >= 180
      ? 0
      : Math.sin(Math.PI * Math.min(1, age / 180)) * Math.exp(-age / 180);
  return {
    recoil: pulse * (kind === "cannon" ? 13 : kind === "arrow" ? 5 : 3),
    glow: pulse,
  };
}
export function workerPose(
  index: number,
  count: number,
  at: number,
  reduced: boolean,
) {
  const angle = -Math.PI + ((index + 0.5) / Math.max(1, count)) * Math.PI * 2;
  return {
    x: Math.cos(angle) * 24,
    y: Math.sin(angle) * 13,
    swing: reduced ? 0 : Math.sin(at / 150 + index * 2.4),
  };
}
export function craterDepth(remaining: number, total: number) {
  return Math.max(0, Math.min(1, remaining / Math.max(1, total)));
}
