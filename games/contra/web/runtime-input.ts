import {
  KeyboardController,
  actionsForKey,
  sampleLogicalControllerButton,
  type ControllerProfile,
} from "../../../web/src/controllers/input";
import { contraControllerMask } from "./controller";
import type { FCControllerSink } from "../../../web/src/controllers/FCController";
import {
  PhysicalController,
  mergeLogicalButtons,
} from "../../../web/src/controllers/gamepad";

export class RuntimeInput implements FCControllerSink {
  private keyboard: KeyboardController;
  private gamepad: PhysicalController;
  private pointers = new Map<
    number,
    { buttons: readonly string[]; began: number }
  >();
  private pending = new Map<string, Set<string>>();
  private keys = new Set<string>();
  private enabled = true;
  constructor(
    private profile: ControllerProfile,
    private root: HTMLElement,
    private release: () => void,
  ) {
    this.keyboard = new KeyboardController(profile);
    this.gamepad = new PhysicalController(profile);
    window.addEventListener("keydown", this.down);
    window.addEventListener("keyup", this.up);
    window.addEventListener("blur", this.cancel);
    document.addEventListener("visibilitychange", this.visibility);
    document.addEventListener("focusin", this.focus);
  }
  private blocked(target: EventTarget | null) {
    return (
      !this.enabled ||
      document.hidden ||
      !!document.querySelector('dialog[open],[aria-modal="true"]') ||
      (target instanceof HTMLElement &&
        !!target.closest('input,textarea,select,[contenteditable="true"]')) ||
      (target instanceof HTMLElement &&
        target !== document.body &&
        !this.root.contains(target) &&
        !(
          target.matches("[data-fullscreen-board]") &&
          target.contains(this.root)
        ))
    );
  }
  private down = (e: KeyboardEvent) => {
    if (
      this.blocked(e.target) ||
      e.defaultPrevented ||
      e.repeat ||
      e.metaKey ||
      e.ctrlKey ||
      e.altKey ||
      this.keys.has(e.code)
    )
      return;
    const buttons = actionsForKey(this.profile, e.code);
    if (!buttons.length) return;
    e.preventDefault();
    this.keys.add(e.code);
    this.keyboard.press(e.code, performance.now());
    this.pending.set(
      "key:" + e.code,
      new Set(
        buttons.map((name) =>
          name === "turboA" ? "a" : name === "turboB" ? "b" : name,
        ),
      ),
    );
  };
  private up = (e: KeyboardEvent) => {
    this.keyboard.release(e.code);
    this.keys.delete(e.code);
  };
  private visibility = () => {
    if (document.hidden) this.cancel();
  };
  private focus = (e: FocusEvent) => {
    if (this.blocked(e.target)) this.cancel();
  };
  cancel = () => {
    this.keyboard.clear();
    this.gamepad.clear();
    this.keys.clear();
    this.pointers.clear();
    this.pending.clear();
    this.release();
  };
  configure(profile: ControllerProfile) {
    this.cancel();
    this.profile = profile;
    this.keyboard.configure(profile);
    this.gamepad.configure(profile);
  }
  setEnabled(enabled: boolean) {
    if (this.enabled && !enabled) this.cancel();
    this.enabled = enabled;
  }
  pressPointer(id: number, buttons: readonly string[]) {
    if (!this.enabled || document.hidden) return;
    this.root.focus({ preventScroll: true });
    const previous = this.pointers.get(id);
    if (previous?.buttons.join(",") === buttons.join(",")) return;
    this.pointers.set(id, { buttons: [...buttons], began: performance.now() });
    this.pending.set(
      "pointer:" + id,
      new Set(
        buttons.map((name) =>
          name === "turboA" ? "a" : name === "turboB" ? "b" : name,
        ),
      ),
    );
  }
  releasePointer(id: number, cancelled: boolean) {
    this.pointers.delete(id);
    if (cancelled) this.pending.delete("pointer:" + id);
  }
  sample(now: number) {
    const buttons = mergeLogicalButtons(
      this.keyboard.sample(now),
      this.gamepad.sample(now, this.enabled && !document.hidden),
    );
    for (const source of this.pointers.values())
      for (const name of source.buttons) {
        const button = sampleLogicalControllerButton(
          name,
          source.began,
          now,
          this.profile.turboHz,
        );
        if (button) buttons.add(button);
      }
    for (const set of this.pending.values())
      for (const button of set) buttons.add(button);
    this.pending.clear();
    return this.enabled && !document.hidden
      ? contraControllerMask(mergeLogicalButtons(buttons))
      : 0;
  }
  dispose() {
    this.cancel();
    this.gamepad.dispose();
    window.removeEventListener("keydown", this.down);
    window.removeEventListener("keyup", this.up);
    window.removeEventListener("blur", this.cancel);
    document.removeEventListener("visibilitychange", this.visibility);
    document.removeEventListener("focusin", this.focus);
  }
}
