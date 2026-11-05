import {
  KeyboardController,
  actionsForKey,
  type ControllerProfile,
} from "../../../web/src/controllers/input";
export const mdBits = {
  up: 1,
  down: 2,
  left: 4,
  right: 8,
  b: 16,
  c: 32,
  a: 64,
  start: 128,
  z: 256,
  y: 512,
  x: 1024,
  mode: 2048,
} as const;
export type MDButton = keyof typeof mdBits;
function check(profile: ControllerProfile) {
  const names = Object.keys(mdBits),
    codes: string[] = [];
  if (
    !profile ||
    !Number.isSafeInteger(profile.revision) ||
    !profile.keyboard ||
    Object.keys(profile.keyboard).length !== names.length ||
    names.some((name) => typeof profile.keyboard[name] !== "string") ||
    Object.keys(profile.keyboardAlt ?? {}).some(
      (name) => !Object.hasOwn(mdBits, name),
    )
  )
    throw Error("MD 控制器配置无效");
  for (const name of names) {
    const primary = profile.keyboard[name],
      alternatives = profile.keyboardAlt?.[name] ?? [];
    if (
      !Array.isArray(alternatives) ||
      alternatives.length > 4 ||
      alternatives.some((code) => typeof code !== "string" || !code)
    )
      throw Error("MD 替代按键无效");
    if (primary) codes.push(primary);
    codes.push(...alternatives);
  }
  if (new Set(codes).size !== codes.length) throw Error("MD 按键冲突");
  return structuredClone(profile);
}
export class MDInput {
  private profile: ControllerProfile;
  private keyboard: KeyboardController;
  private root: HTMLElement;
  private onCancel: (remaining: number) => void;
  private pointers = new Map<number, Set<MDButton>>();
  private taps = new Map<string, Set<MDButton>>();
  private down = new Set<string>();
  private enabled = false;
  private closed = false;
  constructor(
    root: HTMLElement,
    profile: ControllerProfile,
    onCancel: (remaining: number) => void,
  ) {
    this.root = root;
    this.profile = check(profile);
    this.keyboard = new KeyboardController(this.profile, {});
    this.onCancel = onCancel;
    window.addEventListener("keydown", this.keyDown);
    window.addEventListener("keyup", this.keyUp);
    window.addEventListener("blur", this.cancel);
    document.addEventListener("visibilitychange", this.visibility);
    document.addEventListener("focusin", this.focus);
  }
  configure(profile: ControllerProfile) {
    const next = check(profile);
    this.cancel();
    this.profile = next;
    this.keyboard.configure(next);
  }
  setEnabled(value: boolean) {
    if (!value && this.enabled) this.cancel();
    this.enabled = value;
  }
  private blocked(target: EventTarget | null) {
    if (this.closed || !this.enabled || document.hidden) return true;
    if (
      document.querySelector('dialog[open],[role="dialog"][aria-modal="true"]')
    )
      return true;
    if (!(target instanceof HTMLElement)) return true;
    if (
      target.closest(
        'input,select,textarea,[contenteditable]:not([contenteditable="false"])',
      )
    )
      return true;
    return target !== document.body && !this.root.contains(target);
  }
  private keyDown = (event: KeyboardEvent) => {
    if (
      this.blocked(event.target) ||
      event.defaultPrevented ||
      event.isComposing
    )
      return;
    if (
      (event.metaKey || event.ctrlKey || event.altKey) &&
      !/^(Meta|Control|Alt)/.test(event.code)
    )
      return;
    const names = actionsForKey(this.profile, event.code).filter((name) =>
      Object.hasOwn(mdBits, name),
    ) as MDButton[];
    if (!names.length) return;
    event.preventDefault();
    if (event.repeat || this.down.has(event.code)) return;
    this.down.add(event.code);
    this.keyboard.press(event.code, performance.now());
    this.taps.set("key:" + event.code, new Set(names));
  };
  private keyUp = (event: KeyboardEvent) => {
    this.keyboard.release(event.code);
    this.down.delete(event.code);
  };
  private visibility = () => {
    if (document.hidden) this.cancel();
  };
  private focus = (event: FocusEvent) => {
    if (this.blocked(event.target)) this.cancel();
  };
  pressPointer(id: number, buttons: readonly string[]) {
    if (this.closed || !this.enabled || document.hidden) return;
    if (
      !Number.isInteger(id) ||
      buttons.some((name) => !Object.hasOwn(mdBits, name))
    )
      throw Error("MD 触屏输入无效");
    this.root.focus({ preventScroll: true });
    const next = new Set(buttons as MDButton[]),
      old = this.pointers.get(id);
    if (
      old &&
      old.size === next.size &&
      [...next].every((name) => old.has(name))
    )
      return;
    this.pointers.set(id, next);
    this.taps.set("pointer:" + id, next);
  }
  releasePointer(id: number, cancelled = false) {
    if (!this.pointers.delete(id)) return;
    if (cancelled) {
      this.taps.delete("pointer:" + id);
      this.onCancel(this.mask());
    }
  }
  private mask() {
    let result = 0;
    for (const name of this.keyboard.sample(performance.now()))
      if (Object.hasOwn(mdBits, name)) result |= mdBits[name as MDButton];
    for (const source of this.pointers.values())
      for (const name of source) result |= mdBits[name];
    for (const source of this.taps.values())
      for (const name of source) result |= mdBits[name];
    return result;
  }
  sample() {
    if (this.closed || !this.enabled || document.hidden) return 0;
    const result = this.mask();
    this.taps.clear();
    return result;
  }
  cancelLocal() {
    this.keyboard.clear();
    this.down.clear();
    this.pointers.clear();
    this.taps.clear();
  }
  cancel = () => {
    this.cancelLocal();
    this.onCancel(0);
  };
  dispose() {
    if (this.closed) return;
    this.cancel();
    this.closed = true;
    window.removeEventListener("keydown", this.keyDown);
    window.removeEventListener("keyup", this.keyUp);
    window.removeEventListener("blur", this.cancel);
    document.removeEventListener("visibilitychange", this.visibility);
    document.removeEventListener("focusin", this.focus);
  }
}
