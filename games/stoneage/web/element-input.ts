// LOGIN.CPP creation: ten points; earth/fire and water/wind are exclusive.
export function adjustElementPoints(
  values: readonly number[],
  index: number,
  delta: number,
): number[] {
  const next = [...values];
  if (delta < 0) {
    if (next[index] > 0) next[index]--;
    return next;
  }
  if (next[(index + 2) % 4] > 0 || next[index] >= 10) return next;
  if (next.reduce((a, b) => a + b, 0) < 10) {
    next[index]++;
    return next;
  }
  const source = next.findIndex((v, i) => i !== index && v > 0);
  if (source >= 0) {
    next[index]++;
    next[source]--;
  }
  return next;
}
