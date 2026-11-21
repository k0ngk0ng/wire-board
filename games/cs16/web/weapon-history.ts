type Owner = { weapon: string; life: number; appliedSeq: number; inventory?: { id: string }[] };

export class WeaponHistory {
  current = '';
  previous = '';
  private identity = '';
  private life = 0;
  private pending = 0;
  confirm(player: Owner, identity: string, reset = false) {
    if (reset || identity !== this.identity || player.life !== this.life) {
      this.identity = identity;
      this.life = player.life;
      this.current = player.weapon;
      this.previous = '';
      this.pending = 0;
      return;
    }
    if (this.pending > player.appliedSeq && player.inventory?.some(item => item.id === this.current)) return;
    this.pending = 0;
    if (player.weapon !== this.current) { this.previous = this.current; this.current = player.weapon; }
  }
  select(id: string, seq: number, inventory: { id: string }[] | undefined) {
    if (!inventory?.some(item => item.id === id)) return false;
    if (id !== this.current) { this.previous = this.current; this.current = id; }
    this.pending = seq;
    return true;
  }
  reject(seq: number, player: Owner, identity: string) {
    if (seq < this.pending) return false;
    this.confirm(player, identity, true);
    return true;
  }
}
