export type RacingTouchAction =
  "left" | "right" | "throttle" | "brake" | "drift" | "horn" | "rear";
const steering = new Set<RacingTouchAction>(["left", "right"]);
const pedals = new Set<RacingTouchAction>(["throttle", "brake", "drift"]);

// Touch identifiers are independent of PointerEvent.pointerId. The native
// active-touch snapshot is authoritative even when a pointer release is lost.
export class RacingTouchContacts {
  private contacts = new Map<
    number,
    { origin: RacingTouchAction; current: RacingTouchAction | null }
  >();
  press(id: number, action: RacingTouchAction) {
    this.contacts.set(id, { origin: action, current: action });
  }
  has(id: number) {
    return this.contacts.has(id);
  }
  get size() {
    return this.contacts.size;
  }
  move(id: number, action: RacingTouchAction | null) {
    const contact = this.contacts.get(id);
    if (!contact) return;
    const group = steering.has(contact.origin)
      ? steering
      : pedals.has(contact.origin)
        ? pedals
        : new Set([contact.origin]);
    contact.current = action && group.has(action) ? action : null;
  }
  reconcile(active: readonly number[]) {
    const ids = new Set(active);
    for (const id of this.contacts.keys())
      if (!ids.has(id)) this.contacts.delete(id);
  }
  clear() {
    this.contacts.clear();
  }
  get actions(): ReadonlySet<RacingTouchAction> {
    return new Set(
      [...this.contacts.values()].flatMap((contact) =>
        contact.current ? [contact.current] : [],
      ),
    );
  }
}
