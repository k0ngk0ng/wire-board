// Display limits only. The server revalidates every submitted action.
export function recruitLimit(
  gold: number,
  cost: number,
  population: number,
  movement: number,
  required: number,
) {
  if (movement < required || cost <= 0) return 0;
  return Math.max(
    0,
    Math.min(Math.floor(gold / cost), Math.floor(population / 2)),
  );
}
export function moveLimit(
  mobile: number,
  movement: number,
  required: number,
  sea: boolean,
  gold: number,
  seaCost: number,
) {
  if (movement < required) return 0;
  return Math.max(
    0,
    Math.floor(sea && seaCost > 0 ? Math.min(mobile, gold / seaCost) : mobile),
  );
}
export function wholeAmount(value: string) {
  const n = Number(value);
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : 0;
}
