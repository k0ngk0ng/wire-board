type PlayerCapacity = {
  minPlayers: number;
  maxPlayers: number;
  playerCounts?: readonly number[];
  defaultPlayers?: number;
};

export function gameDefaultPlayers(module: PlayerCapacity): number {
  const minimum = Number.isSafeInteger(module.minPlayers) ? module.minPlayers : 1;
  if (!Number.isSafeInteger(module.minPlayers) || !Number.isSafeInteger(module.maxPlayers)) return minimum;
  const counts = module.playerCounts ?? [];
  if (!counts.every(Number.isSafeInteger)) return minimum;
  const selected = module.defaultPlayers;
  if (selected !== undefined && Number.isSafeInteger(selected) && selected >= module.minPlayers && selected <= module.maxPlayers && (!module.playerCounts || counts.includes(selected))) return selected;
  return minimum;
}
