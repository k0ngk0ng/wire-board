import { moveLimit, recruitLimit } from "./action-limits.ts";

// null keeps the default tied to the latest authoritative capacity. An explicit
// choice is captured with each queued click, then bounded by remaining troops.
export function troopAmount(
  maximum: number,
  requested: number | null = null,
): number {
  if (!Number.isFinite(maximum) || maximum <= 0) return 0;
  const limit = Math.floor(maximum);
  if (requested === null) return Math.ceil(limit / 2);
  if (!Number.isFinite(requested)) return 0;
  return Math.max(0, Math.min(limit, Math.floor(requested)));
}

export function marchAmount(
  mobile: number,
  gold: number,
  movement: number,
  sea: boolean,
  seaCost: number,
  moveCost: number,
  requested: number | null = null,
): number {
  const amount = troopAmount(mobile, requested);
  return moveLimit(mobile, movement, moveCost, sea, gold, seaCost) >= amount
    ? amount
    : 0;
}

export function recruitCapacity(
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
      999999 - troops,
      recruitLimit(gold, unitCost, population, movement, actionCost),
    ),
  );
}

export function recruitShortages(
  gold: number,
  population: number,
  troops: number,
  movement: number,
  unitCost: number,
  actionCost: number,
): Array<"movement" | "gold" | "population" | "capacity"> {
  const reasons: Array<"movement" | "gold" | "population" | "capacity"> = [];
  if (movement < actionCost) reasons.push("movement");
  if (gold < unitCost) reasons.push("gold");
  if (population < 2) reasons.push("population");
  if (troops >= 999999) reasons.push("capacity");
  return reasons;
}
