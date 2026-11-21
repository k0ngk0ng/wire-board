import type { Player } from './types.ts';

export type MotionCollider = Pick<Player, 'seat' | 'alive' | 'x' | 'y' | 'z' | 'crouch'>;

export function motionColliders(players: readonly Player[]): MotionCollider[] {
  return players.filter(player => player.visible !== false && Number.isFinite(player.x) && Number.isFinite(player.y) && Number.isFinite(player.z)).map(({ seat, alive, x, y, z, crouch }) => ({ seat, alive, x, y, z, crouch }));
}
