type PlayerCapacity = {
  minPlayers: number;
  maxPlayers: number;
  playerCounts?: readonly number[];
  defaultPlayers?: number;
};

export function gameDefaultPlayers(module: PlayerCapacity): number {
  const minimum = Number.isSafeInteger(module.minPlayers) ? module.minPlayers : 1;
  if (!Number.isSafeInteger(module.minPlayers) || !Number.isSafeInteger(module.maxPlayers)) return minimum;
  if (module.maxPlayers < minimum) return minimum;
  if (!module.playerCounts) return module.maxPlayers;
  let selected = minimum;
  for (const count of module.playerCounts) {
    if (!Number.isSafeInteger(count)) return minimum;
    if (count < module.minPlayers || count > module.maxPlayers) continue;
    selected = Math.max(selected, count);
  }
  return selected;
}

export function formatPlayerCounts(module: PlayerCapacity): string {
  const min = Number.isSafeInteger(module.minPlayers) ? module.minPlayers : 1;
  const max = Number.isSafeInteger(module.maxPlayers) && module.maxPlayers >= min ? module.maxPlayers : min;
  if (module.playerCounts) {
    const seen = new Set<number>();
    for (const count of module.playerCounts) {
      if (Number.isSafeInteger(count) && count >= min && count <= max) seen.add(count);
    }
    const valid: string[] = [];
    let first = 0;
    let last = 0;
    let total = 0;
    for (let value = min; value <= max; value += 1) {
      if (!seen.has(value)) continue;
      if (total === 0) first = value;
      last = value;
      total += 1;
      valid.push(String(value));
    }
    if (total === 1) return String(first);
    if (total > 1) {
      return last - first + 1 === total ? `${first}～${last}` : valid.join(" / ");
    }
  }
  return min === max ? String(min) : `${min}～${max}`;
}
