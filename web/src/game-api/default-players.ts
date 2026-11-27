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
  if (module.playerCounts?.length) {
    const valid = [...new Set(module.playerCounts.filter((count) => Number.isSafeInteger(count) && count >= min && count <= max))].sort((a, b) => a - b);
    if (valid.length === 1) return String(valid[0]);
    if (valid.length > 1) {
      const contiguous = valid.every((count, index) => index === 0 || count === valid[index - 1] + 1);
      if (contiguous) return `${valid[0]}～${valid[valid.length - 1]}`;
      return valid.join(" / ");
    }
  }
  return min === max ? String(min) : `${min}～${max}`;
}
