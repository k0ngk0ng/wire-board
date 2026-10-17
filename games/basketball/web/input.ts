import { KeyboardController, sampleLogicalControllerButton, type ControllerProfile } from '../../../web/src/controllers/input';

export type BasketballButton = 'right' | 'left' | 'down' | 'up' | 'start' | 'select' | 'b' | 'a';
const bits: Record<BasketballButton, number> = { right:1,left:2,down:4,up:8,start:16,select:32,b:64,a:128 };
const logical = (name: string): BasketballButton | undefined => {
  if (name === 'turboA') return 'a';
  if (name === 'turboB') return 'b';
  return Object.hasOwn(bits, name) ? name as BasketballButton : undefined;
};
function checkedProfile(profile: ControllerProfile): ControllerProfile {
  if (!Number.isInteger(profile.turboHz) || profile.turboHz < 2 || profile.turboHz > 15) throw new Error('Invalid controller turbo rate');
  const keys = Object.values(profile.keyboard);
  if (keys.some(key => typeof key !== 'string' || !key) || new Set(keys).size !== keys.length) throw new Error('Invalid controller key mapping');
  return structuredClone(profile);
}

/** One user's current controller profile. Persistence belongs to the platform. */
export class BasketballInput {
  private profile: ControllerProfile;
  private keyboard: KeyboardController;
  private keys = new Set<string>();
  private pointers = new Map<number, {buttons:Set<string>;began:number}>();
  private pending = new Map<string, Set<BasketballButton>>();
  private enabled = true;
  private disposed = false;
  private releases = new Set<() => void>();

  constructor(profile: ControllerProfile, private root: HTMLElement, private onCancel?: (mask:number)=>void) {
    this.profile = checkedProfile(profile);
    this.keyboard = new KeyboardController(this.profile);
    window.addEventListener('keydown', this.keyDown);
    window.addEventListener('keyup', this.keyUp);
    window.addEventListener('blur', this.cancel);
    document.addEventListener('visibilitychange', this.visibility);
    document.addEventListener('focusin', this.focusChanged);
  }

  configure(profile: ControllerProfile) {
    const next = checkedProfile(profile);
    this.cancel();
    this.profile = next;
    this.keyboard.configure(next);
  }

  setEnabled(value: boolean) {
    if (!value) this.cancel();
    this.enabled = value;
  }

  cancel = () => {
    this.cancelLocal();
    this.onCancel?.(255);
  };

  /** Restore may discard local physical sources without rewriting a saved
   * engine's already-latched input. Ordinary focus/device cancellation notifies.
   */
  cancelLocal() {
    this.keyboard.clear();
    this.keys.clear();
    this.pointers.clear();
    this.pending.clear();
  }

  private visibility = () => { if (document.hidden) this.cancel(); };
  private focusChanged = (event:FocusEvent) => { if(this.blocked(event.target))this.cancel(); };

  private blocked(target: EventTarget | null): boolean {
    if (!this.enabled || this.disposed || document.hidden) return true;
    if (document.querySelector('dialog[open], [role="dialog"][aria-modal="true"]')) return true;
    if (!(target instanceof HTMLElement)) return true;
    if (target.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])')) return true;
    if (target.closest('button, a[href]') && !this.root.contains(target)) return true;
    return target !== document.body && !this.root.contains(target);
  }

  private keyDown = (event: KeyboardEvent) => {
    if (this.blocked(event.target) || event.defaultPrevented || event.isComposing) return;
    if ((event.metaKey || event.ctrlKey || event.altKey) && !/^(Meta|Control|Alt)/.test(event.code)) return;
    const names = Object.entries(this.profile.keyboard).filter(([,code]) => code === event.code).flatMap(([name]) => {
      const button = logical(name); return button ? [button] : [];
    });
    if (!names.length) return;
    event.preventDefault();
    if (event.repeat || this.keys.has(event.code)) return;
    this.keys.add(event.code);
    this.keyboard.press(event.code, performance.now());
    this.pending.set(`key:${event.code}`, new Set(names));
  };

  private keyUp = (event: KeyboardEvent) => {
    this.keyboard.release(event.code);
    this.keys.delete(event.code);
  };

  pressPointer(id: number, buttons: readonly string[], at = performance.now()) {
    if (!this.enabled || this.disposed || document.hidden) return;
    if (!Number.isInteger(id) || buttons.some(button => !logical(button))) throw new Error('Invalid controller pointer');
    if(!buttons.length){this.releasePointer(id);return;}
    const held = new Set(buttons);
    const previous=this.pointers.get(id);
    if(previous&&previous.buttons.size===held.size&&buttons.every(button=>previous.buttons.has(button)))return;
    this.pointers.set(id,{buttons:held,began:at});
    this.pending.set(`pointer:${id}`,new Set(buttons.map(button=>logical(button)!)));
  }

  releasePointer(id: number, cancelled = false) {
    const buttons=this.pointers.get(id)?.buttons;
    const active = this.pointers.delete(id);
    // A normal pointerup keeps a short tap until the next video sample.
    // The subsequent lostpointercapture must not turn it into cancellation.
    if (cancelled && active) {
      this.pending.delete(`pointer:${id}`);
      let mask=0;for(const button of buttons!)mask|=bits[logical(button)!];
      mask &= ~this.heldMask(performance.now());
      if(mask)this.onCancel?.(mask);
    }
  }

  bindButton(element: HTMLElement, buttons: readonly BasketballButton[]): () => void {
    const oldTouchAction = element.style.touchAction;
    element.style.touchAction = 'none';
    const owned = new Set<number>();
    const down = (event: PointerEvent) => {
      if (event.button !== 0 || this.blocked(event.target)) return;
      event.preventDefault();
      this.root.focus({preventScroll:true});
      this.pressPointer(event.pointerId,buttons);
      owned.add(event.pointerId);
      element.setPointerCapture(event.pointerId);
    };
    const release = (event: PointerEvent) => {
      if (!owned.delete(event.pointerId)) return;
      this.releasePointer(event.pointerId,event.type !== 'pointerup');
    };
    element.addEventListener('pointerdown',down);
    element.addEventListener('pointerup',release);
    element.addEventListener('pointercancel',release);
    element.addEventListener('lostpointercapture',release);
    const unbind = () => {
      for (const id of owned) this.releasePointer(id,true);
      owned.clear();
      element.removeEventListener('pointerdown',down);
      element.removeEventListener('pointerup',release);
      element.removeEventListener('pointercancel',release);
      element.removeEventListener('lostpointercapture',release);
      element.style.touchAction = oldTouchAction;
      this.releases.delete(unbind);
    };
    this.releases.add(unbind);
    return unbind;
  }

  /** Sample once per video tick. Keyboard and pointers remain independent. */
  sample(at = performance.now()): number {
    if (!this.enabled || this.disposed || document.hidden) return 0;
    const held=this.heldMask(at);
    this.pending.clear();
    return held;
  }

  private heldMask(at:number):number {
    let held=0;
    for (const name of this.keyboard.sample(at)) { const button = logical(name); if (button) held |= bits[button]; }
    for (const source of this.pointers.values()) for (const name of source.buttons) {
      const active=sampleLogicalControllerButton(name,source.began,at,this.profile.turboHz);
      if(active){const button=logical(active);if(button)held|=bits[button];}
    }
    for (const source of this.pending.values()) for (const button of source) held |= bits[button];
    return held;
  }

  dispose() {
    if (this.disposed) return;
    this.cancel();
    this.disposed = true;
    window.removeEventListener('keydown',this.keyDown);
    window.removeEventListener('keyup',this.keyUp);
    window.removeEventListener('blur',this.cancel);
    document.removeEventListener('visibilitychange',this.visibility);
    document.removeEventListener('focusin',this.focusChanged);
    for (const unbind of [...this.releases]) unbind();
  }
}
