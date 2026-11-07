import type { Passive } from "./types";
export function skillProgress(
  passive: Pick<Passive, "cooldownMS" | "readyInMS" | "stacks" | "maxStacks">,
  elapsedMS: number,
  running: boolean,
) {
  const finite = (value: number) =>
    Number.isFinite(value) ? Math.max(0, value) : 0;
  if (passive.maxStacks > 0) {
    const progress = Math.min(1, finite(passive.stacks) / passive.maxStacks);
    return { progress, remainingMS: 0, ready: progress >= 1, stacks: true };
  }
  const remainingMS = Math.max(
    0,
    finite(passive.readyInMS) - (running ? finite(elapsedMS) : 0),
  );
  const duration = finite(passive.cooldownMS);
  return {
    progress: duration
      ? Math.max(0, Math.min(1, 1 - remainingMS / duration))
      : 1,
    remainingMS,
    ready: remainingMS === 0,
    stacks: false,
  };
}
