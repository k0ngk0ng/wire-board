/** Reorder only visible slots; filtering never drops or relocates other games. */
export function reorderVisible(
  all: readonly string[],
  visible: readonly string[],
  from: string,
  to: string,
): string[] {
  const first = visible.indexOf(from),
    last = visible.indexOf(to);
  if (first < 0 || last < 0 || first === last) return [...all];
  const moved = [...visible];
  moved.splice(first, 1);
  moved.splice(last, 0, from);
  const active = new Set(visible);
  let i = 0;
  return all.map((id) => (active.has(id) ? moved[i++] : id));
}
