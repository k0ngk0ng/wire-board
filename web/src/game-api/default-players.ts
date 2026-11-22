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
