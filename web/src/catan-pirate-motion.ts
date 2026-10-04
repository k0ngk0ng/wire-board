import type { CatanState } from "./types";

// Only animate a single newly observed roll. An initial view, reconnect gap,
// rematch or removed fleet must render the authoritative destination directly.
export function catanFleetSteps(
  before: CatanState,
  after: CatanState,
): number[] {
  const old = before.seafarers,
    next = after.seafarers;
  const path = next?.pirateIslands?.fleetPath;
  if (
    !old?.pirateIslands ||
    !next ||
    !path?.length ||
    after.rollId !== before.rollId + 1 ||
    old.pirate < 0 ||
    next.pirate < 0 ||
    path.join(",") !== old.pirateIslands.fleetPath.join(",")
  )
    return [];
  const from = path.indexOf(old.pirate),
    count = Math.min(...after.dice);
  if (
    from < 0 ||
    after.dice.length !== 2 ||
    count < 1 ||
    count > 6 ||
    after.dice.some((n) => !Number.isInteger(n) || n < 1 || n > 6) ||
    path[(from + count) % path.length] !== next.pirate
  )
    return [];
  return Array.from(
    { length: count + 1 },
    (_, i) => path[(from + i) % path.length],
  );
}

export function catanNewBattle(before: CatanState, after: CatanState) {
  const battle = after.seafarers?.pirateIslands?.battle;
  if (
    !battle ||
    battle.id !== (before.seafarers?.pirateIslands?.battle?.id ?? 0) + 1 ||
    before.rollId !== after.rollId
  )
    return undefined;
  return battle;
}
