type PlayerCapacity = {
  minPlayers: number;
  maxPlayers: number;
  playerCounts?: readonly number[];
  defaultPlayers?: number;
};

export function gameDefaultPlayers(module: PlayerCapacity): number {
  const selected = module.defaultPlayers;
  if (selected !== undefined && Number.isInteger(selected) && selected >= module.minPlayers && selected <= module.maxPlayers && (!module.playerCounts || module.playerCounts.includes(selected))) return selected;
  return module.minPlayers;
}
