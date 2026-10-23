import { moveLimit, recruitLimit } from "./action-limits.ts";

export function halfMarch(
  mobile: number,
  gold: number,
  movement: number,
  sea: boolean,
  seaCost: number,
  moveCost: number,
): number {
  if (!Number.isFinite(mobile) || mobile <= 0) return 0;
  const half = Math.ceil(mobile / 2);
  return moveLimit(mobile, movement, moveCost, sea, gold, seaCost) >= half
    ? half
    : 0;
}

export function quickRecruit(
  gold: number,
  population: number,
  troops: number,
  movement: number,
  unitCost: number,
  actionCost: number,
): number {
  return Math.max(
    0,
    Math.min(
      1000,
      999999 - troops,
      recruitLimit(gold, unitCost, population, movement, actionCost),
    ),
  );
}
