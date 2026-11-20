export type WeaponTiming = { id: string; cooldown: number; melee?: boolean };
type AttackPlayer = { life: number; weapon: string; ammo: number; alive: boolean; nextAttackFrame: number; downUntil: number; busyUntil?: number; appliedSeq: number };

export class AttackPresentation {
  private weapons: Map<string, WeaponTiming>;
  private player: AttackPlayer | undefined;
  private pending: { seq: number; frame: number }[] = [];

  constructor(weapons: WeaponTiming[]) {
    this.weapons = new Map(weapons.map(weapon => [weapon.id, weapon]));
  }

  confirm(player: AttackPlayer) {
    if (player.life !== this.player?.life || player.weapon !== this.player?.weapon) this.pending = [];
    else this.pending = this.pending.filter(shot => shot.seq > player.appliedSeq);
    this.player = { ...player };
  }

  fire(seq: number, frame: number) {
    const player = this.player;
    const weapon = player && this.weapons.get(player.weapon);
    if (!player?.alive || !weapon || frame < player.downUntil || frame < (player.busyUntil ?? 0)) return false;
    const nextAttackFrame = this.pending.reduce((next, shot) => Math.max(next, shot.frame + weapon.cooldown), player.nextAttackFrame);
    if (frame < nextAttackFrame || !weapon.melee && player.ammo <= this.pending.length) return false;
    this.pending.push({ seq, frame });
    if (this.pending.length > 32) this.pending.shift();
    return true;
  }

  clear() {
    this.pending = [];
  }
}
