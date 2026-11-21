import { TouchInput, type TouchState } from './touch-input.ts';

type CaptureTarget = { setPointerCapture: (id: number) => void; hasPointerCapture: (id: number) => boolean; releasePointerCapture: (id: number) => void };
type Contact = { target: CaptureTarget; captured: boolean; raw: boolean };
type TouchCallbacks = { locked: () => boolean; begin: () => void; changed: (state: TouchState) => void; look: (x: number, y: number) => void; interrupted: () => void; failed: (error: unknown) => void };

export class PointerLockHandoff {
  private transitioning = false;
  get pending() { return this.transitioning; }
  begin(locked: boolean, release: () => void, unlock: () => void) {
    if (!locked || this.transitioning) return;
    release();
    this.transitioning = true;
    try { unlock(); }
    catch (error) { this.transitioning = false; throw error; }
  }
  lockChanged(locked: boolean) {
    if (locked) return false;
    const clear = !this.transitioning;
    this.transitioning = false;
    return clear;
  }
  reset() { this.transitioning = false; }
}

export class TouchOwnership {
  private input = new TouchInput();
  private contacts = new Map<number, Contact>();
  private callbacks: TouchCallbacks;
  constructor(callbacks: TouchCallbacks) { this.callbacks = callbacks; }
  owns(id: number) { return this.contacts.has(id); }
  sample() { return this.input.sample(); }
  private publish() { this.callbacks.changed(this.input.sample()); }
  press(id: number, kind: string, x: number, y: number, target: CaptureTarget) {
    if (this.owns(id)) return;
    try { this.callbacks.begin(); }
    catch (error) { this.callbacks.failed(error); return; }
    this.contacts.set(id, { target, captured: false, raw: false });
    this.input.press(id, kind, x, y);
    this.publish();
    this.capture(id);
  }
  move(id: number, x: number, y: number, raw = false) {
    const contact = this.contacts.get(id);
    if (!contact || contact.raw && !raw) return;
    if (raw) contact.raw = true;
    const delta = this.input.move(id, x, y);
    if (delta) this.callbacks.look(delta.x, delta.y);
    else this.publish();
  }
  release(id: number, cancelled = false) {
    const contact = this.contacts.get(id);
    if (!contact) return;
    this.contacts.delete(id);
    const kind = this.input.release(id, cancelled);
    if (cancelled && kind === 'fire') this.callbacks.interrupted();
    this.releaseCapture(id, contact);
    this.publish();
  }
  lost(id: number) {
    const contact = this.contacts.get(id);
    if (contact?.captured && !contact.target.hasPointerCapture(id)) this.release(id, true);
  }
  capturePending() {
    if (this.callbacks.locked()) return;
    for (const id of this.contacts.keys()) this.capture(id);
  }
  private capture(id: number) {
    const contact = this.contacts.get(id);
    if (!contact || contact.captured || this.callbacks.locked()) return;
    try { contact.target.setPointerCapture(id); contact.captured = true; }
    catch (error) { this.release(id, true); this.callbacks.failed(error); }
  }
  private releaseCapture(id: number, contact: Contact) {
    try { if (contact.target.hasPointerCapture(id)) contact.target.releasePointerCapture(id); }
    catch (error) { this.callbacks.failed(error); }
  }
  clear(notify = true) {
    const fire = this.input.sample().actions.has('fire');
    const contacts = [...this.contacts];
    this.contacts.clear();
    this.input.clear();
    if (fire && notify) this.callbacks.interrupted();
    for (const [id, contact] of contacts) this.releaseCapture(id, contact);
    if (notify) this.publish();
  }
}

export function bindTouchOwnership(target: EventTarget, lockTarget: EventTarget, input: TouchOwnership, disabled: () => boolean, native: (event: Event) => void) {
  const move = (event: Event) => {
    const pointer = event as PointerEvent;
    if (disabled() || !input.owns(pointer.pointerId)) return;
    native(event);
    event.preventDefault();
    input.move(pointer.pointerId, pointer.clientX, pointer.clientY, event.type === 'pointerrawupdate');
  };
  const release = (event: Event) => {
    const pointer = event as PointerEvent;
    if (!input.owns(pointer.pointerId)) return;
    event.preventDefault();
    input.release(pointer.pointerId, event.type === 'pointercancel');
  };
  const lost = (event: Event) => input.lost((event as PointerEvent).pointerId);
  const capture = () => input.capturePending();
  target.addEventListener('pointermove', move, { capture: true, passive: false });
  target.addEventListener('pointerrawupdate', move, { capture: true, passive: false });
  target.addEventListener('pointerup', release, true);
  target.addEventListener('pointercancel', release, true);
  target.addEventListener('lostpointercapture', lost, true);
  lockTarget.addEventListener('pointerlockchange', capture);
  return () => {
    target.removeEventListener('pointermove', move, true);
    target.removeEventListener('pointerrawupdate', move, true);
    target.removeEventListener('pointerup', release, true);
    target.removeEventListener('pointercancel', release, true);
    target.removeEventListener('lostpointercapture', lost, true);
    lockTarget.removeEventListener('pointerlockchange', capture);
    input.clear(false);
  };
}
