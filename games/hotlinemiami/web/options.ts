import { characterNames, type GameOptions } from './types.ts';

export function resizeOptions(value: GameOptions, capacity: number): GameOptions {
  const available = Object.keys(characterNames);
  return {
    ...value,
    characters: Array.from({ length: capacity }, (_, seat) => value.characters[seat] ?? available[seat % available.length]),
    weapons: Array.from({ length: capacity }, (_, seat) => value.weapons[seat] ?? 'pistol'),
  };
}
