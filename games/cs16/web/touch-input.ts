export type TouchState = { moveX: number; moveY: number; actions: Set<string> };
type Contact = { kind: string; x: number; y: number; currentX: number; currentY: number };

export class TouchInput {
  private contacts = new Map<number, Contact>();
  private crouch = false;
  press(id: number, kind: string, x: number, y: number) {
    if (kind === 'crouch') this.crouch = !this.crouch;
    this.contacts.set(id, { kind, x, y, currentX: x, currentY: y });
  }
  move(id: number, x: number, y: number) {
    const contact = this.contacts.get(id);
    if (!contact) return;
    const delta = contact.kind === 'look' || contact.kind === 'fire' ? { x: x - contact.currentX, y: y - contact.currentY } : undefined;
    contact.currentX = x;
    contact.currentY = y;
    return delta;
  }
  release(id: number, cancelled = false) {
    const kind = this.contacts.get(id)?.kind;
    this.contacts.delete(id);
    if (cancelled && kind === 'crouch') this.crouch = false;
    return kind;
  }
  clear() { this.contacts.clear(); this.crouch = false; }
  sample(): TouchState {
    const state: TouchState = { moveX: 0, moveY: 0, actions: new Set(this.crouch ? ['crouch'] : []) };
    for (const contact of this.contacts.values()) {
      if (contact.kind === 'move') {
        const x = Math.round(Math.max(-1000, Math.min(1000, (contact.currentX - contact.x) * 1000 / 48)));
        const y = Math.round(Math.max(-1000, Math.min(1000, (contact.y - contact.currentY) * 1000 / 48)));
        const length = Math.max(1000, Math.hypot(x, y));
        state.moveX = Math.round(x * 1000 / length);
        state.moveY = Math.round(y * 1000 / length);
      } else if (contact.kind !== 'look' && contact.kind !== 'crouch') state.actions.add(contact.kind);
    }
    return state;
  }
}
